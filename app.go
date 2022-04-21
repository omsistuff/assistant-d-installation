package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/gorilla/websocket"
	"github.com/inconshreveable/go-update"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

var name string = "assistant-d-installation"
var cdnPath string = "https://storage.googleapis.com/omsistuff-cdn/programs/autodl/repaints/"
var executable string = "https://firebasestorage.googleapis.com/v0/b/objects-omsistuff.appspot.com/o/programs%2F" + name + ".exe"

var isShutdown bool = false
var canShutdown bool = false

type FirebaseStorage struct {
    Md5Hash string
}

func getJson(url string, target interface{}) error {
    var client = &http.Client{Timeout: 10 * time.Second}

    r, err := client.Get(url)
    if err != nil {
        return err
    }
    defer r.Body.Close()

    return json.NewDecoder(r.Body).Decode(target)
}

func getLastChecksum() string {
    firebaseStorage := FirebaseStorage{}
    getJson(executable, &firebaseStorage)
    return firebaseStorage.Md5Hash
}

func isLastVersion() bool {
    hashFile := fmt.Sprintf(".%v.md5", name)

    localChecksum := ""
    data, err := ioutil.ReadFile(hashFile)
    if err == nil {
        localChecksum = string(data)
        fmt.Printf("Local checksum: %v\n", localChecksum)
    }

    onlineChecksum := getLastChecksum()

    file, err := os.Create(hashFile)

    if err != nil {
        log.Fatalf("failed creating file: %s", err)
    }

    defer file.Close()

    _, err = file.WriteString(onlineChecksum)

    fmt.Printf("Online checksum: %v\n", onlineChecksum)

    if err != nil {
        log.Fatalf("failed writing to file: %s", err)
    }

    return localChecksum == onlineChecksum
}

func doUpdate() {
    if isLastVersion() {
        fmt.Println("No update available")
        return
    }

    fmt.Println("New update available")

    resp, err := http.Get(executable + "?alt=media")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    err = update.Apply(resp.Body, update.Options{})
    if err != nil {
        log.Fatal(err)
    }
}

func awaitMessage(conn *websocket.Conn) (msg string, err error) {
    var timeout = time.Now().Unix() + 15

    for {
        _, p, err := conn.ReadMessage()

        if err != nil {
            return err.Error(), err
        }

        if p != nil {
            return string(p), nil
        }

        if time.Now().Unix() > timeout {
            err = errors.New("Timeout")
            return err.Error(), err
        }
    }
}

func downloadFile(url string, conn *websocket.Conn) {
    // create client
    client := grab.NewClient()
    req, _ := grab.NewRequest(".", url)

    // start download
    fmt.Printf("Downloading %v...\n", req.URL())
    resp := client.Do(req)
    fmt.Printf("  %v\n", resp.HTTPResponse.Status)

    // start UI loop
    t := time.NewTicker(500 * time.Millisecond)
    defer t.Stop()

Loop:
    for {
        select {
        case <-t.C:
            conn.WriteMessage(1, []byte(fmt.Sprintf("%.2f", 100*resp.Progress())))

        case <-resp.Done:
            // download is complete
            break Loop
        }
    }

    // check for errors
    if err := resp.Err(); err != nil {
        fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
        exit()
    }

    fmt.Printf("Download saved to ./%v \n", resp.Filename)
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
    canShutdown = false

    upgrader.CheckOrigin = func(r *http.Request) bool { return true }

    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println(err)
    }

    log.Println("Client Connected")
    err = ws.WriteMessage(1, []byte("Hi Client!"))
    if err != nil {
        log.Println(err)
    }

    ws.WriteMessage(1, []byte("tld:request_download_link"))

    msg, err := awaitMessage(ws)

    if err != nil {
        log.Println(err)
        return
    }

    if strings.HasPrefix(msg, cdnPath) {
        downloadFile(msg, ws)
    }

    canShutdown = true

    exit()
}

func exit() {
    if isShutdown || !canShutdown {
        return
    }

    isShutdown = true

    doUpdate()
    os.Exit(0)
}

func main() {
    fmt.Printf("Assistant d'installation version - (c) Omsistuff 2022\n")
    fmt.Println("Starting local server on port 5300")

    http.HandleFunc("/", wsEndpoint)

    srv := &http.Server{
        Addr: ":5300",
    }
    
    go func() {
        httpError := srv.ListenAndServe()
        if httpError != nil {
            log.Println("While serving HTTP: ", httpError)
        }
    }()
    
    time.Sleep(time.Second * 15)
    srv.Shutdown(context.Background())
    exit()
}
