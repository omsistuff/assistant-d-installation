package main

import (
	"archive/zip"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/gorilla/websocket"
	"github.com/inconshreveable/go-update"
	"github.com/otiai10/copy"
	"github.com/pkg/browser"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

var name string = "assistant-d-installation"
var verifierUrl string = "https://omsistuff.fr/external/autodl/isVerifiedFile?md5="
var executable string = "https://firebasestorage.googleapis.com/v0/b/objects-omsistuff.appspot.com/o/programs%2F" + name + ".exe"

var isShutdown bool = false
var canShutdown bool = true

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
        log.Println("Local checksum:", localChecksum)
    }

    onlineChecksum := getLastChecksum()

    file, err := os.Create(hashFile)

    if err != nil {
        log.Println("failed creating file:", err)
        exit("failed_creating_file")
    }

    defer file.Close()

    _, err = file.WriteString(onlineChecksum)

    log.Println("Online checksum:", onlineChecksum)

    if err != nil {
        log.Println("failed writing to file:", err)
        exit("failed_writing_to_file")
    }

    return localChecksum == onlineChecksum
}

func doUpdate() {
    if isLastVersion() {
        log.Println("No update available")
        return
    }

    log.Println("New update available")

    resp, err := http.Get(executable + "?alt=media")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    err = update.Apply(resp.Body, update.Options{})
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Update applied")
}

func Unzip(src, dest string) error {
    r, err := zip.OpenReader(src)
    if err != nil {
        return err
    }
    defer func() {
        if err := r.Close(); err != nil {
            panic(err)
        }
    }()

    os.MkdirAll(dest, 0755)

    // Closure to address file descriptors issue with all the deferred .Close() methods
    extractAndWriteFile := func(f *zip.File) error {
        rc, err := f.Open()
        if err != nil {
            return err
        }
        defer func() {
            if err := rc.Close(); err != nil {
                panic(err)
            }
        }()

        path := filepath.Join(dest, f.Name)

        // Check for ZipSlip (Directory traversal)
        if !strings.HasPrefix(path, filepath.Clean(dest) + string(os.PathSeparator)) {
            log.Println("illegal file path:", path)
        }

        if f.FileInfo().IsDir() {
            os.MkdirAll(path, f.Mode())
        } else {
            os.MkdirAll(filepath.Dir(path), f.Mode())
            f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
            if err != nil {
                return err
            }
            defer func() {
                if err := f.Close(); err != nil {
                    panic(err)
                }
            }()

            _, err = io.Copy(f, rc)
            if err != nil {
                return err
            }
        }
        return nil
    }

    for _, f := range r.File {
        err := extractAndWriteFile(f)
        if err != nil {
            return err
        }
    }

    return nil
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

func sendMessage(conn *websocket.Conn, msg string) {
    conn.WriteMessage(1, []byte(msg))
}

func downloadFile(url string, conn *websocket.Conn) string {
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
            sendMessage(conn, fmt.Sprintf("download:progress=%.2f", 100*resp.Progress()))

        case <-resp.Done:
            // download is complete
            break Loop
        }
    }

    // check for errors
    if err := resp.Err(); err != nil {
        log.Println("Download failed:", err)
        exit("download_failed")
    }

    return resp.Filename
}

func isVerifiedFile(filename string) bool {
    f, err := os.Open(filename)
    if err != nil {
        log.Fatal(err)
    }

    defer f.Close()
    h := md5.New()
    if _, err := io.Copy(h, f); err != nil {
        log.Fatal(err)
    }
    hash := fmt.Sprintf("%x", h.Sum(nil))

    url := verifierUrl + hash

    log.Println("verify file hash:", url)

    response, err := http.Get(url)
    if err != nil {
        log.Fatal(err)
    }
    defer response.Body.Close()

    responseData, err := ioutil.ReadAll(response.Body)
    if err != nil {
        log.Fatal(err)
    }

    return string(responseData) == "true"
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
    canShutdown = false

    upgrader.CheckOrigin = func(r *http.Request) bool { return true }

    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println(err)
    }

    sendMessage(ws, "need:download_link")
    msg, err := awaitMessage(ws)

    if err != nil {
        log.Println(err)
        return
    }

    sendMessage(ws, "download:start")
    fileName := downloadFile(msg, ws)

    if (!isVerifiedFile(fileName)) {
        os.Remove(fileName)
        sendMessage(ws, "err:must_revalidate")
        exit("must_revalidate")
        return
    }

    sendMessage(ws, "archive:start")
    tmpFolder := ".fr.omsistuff.tmp"
    err = Unzip(fileName, tmpFolder)

    if err != nil {
        log.Println(err)
        exit("unzip_error")
    }

    copy.Copy(tmpFolder + "/OMSI 2/vehicles/", "./vehicles/")
    os.RemoveAll(tmpFolder + "/")
    os.Remove(fileName)

    canShutdown = true
    sendMessage(ws, "archive:done")
    exit()
}

func exit(errCode ...string) {
    fmt.Println(canShutdown)

    if isShutdown || !canShutdown {
        return
    }

    isShutdown = true

    if len(errCode) > 0 {
        browser.OpenURL("https://omsistuff.fr/assistant-d-installation?error=" + errCode[0])
    }

    doUpdate()
    os.Exit(0)
}

func main() {

    LOG_FILE := name + ".log"

    // delete previous logfile
    if _, err := os.Stat(LOG_FILE); err == nil {
        os.Remove(LOG_FILE)
    }

    // open log file
    logFile, err := os.OpenFile(LOG_FILE, os.O_RDWR|os.O_CREATE, 0644)
    if err != nil {
        log.Panic(err)
    }
    defer logFile.Close()

    // Set log out put and enjoy :)
    log.SetOutput(logFile)

    // optional: log date-time, filename, and line number
    log.SetFlags(log.Lshortfile | log.LstdFlags)

    fmt.Printf("Assistant d'installation (c) Omsistuff 2022\n")
    fmt.Println("Starting local server on port 5300")

    http.HandleFunc("/", wsEndpoint)

    srv := &http.Server{
        Addr: ":5300",
    }
    
    go func() {
        httpError := srv.ListenAndServe()
        if httpError != nil {
            log.Println("While serving HTTP:", httpError)
        }
    }()
    
    for {
        // try to close program every 15 seconds
        time.Sleep(time.Second * 15)
        exit()
    }
}
