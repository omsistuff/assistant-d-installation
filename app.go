package main

import (
    "errors"
    "fmt"
    "github.com/blang/semver"
    "github.com/cavaliergopher/grab/v3"
    "github.com/gorilla/websocket"
    "github.com/rhysd/go-github-selfupdate/selfupdate"
    "log"
    "net/http"
    "os"
    "strings"
    "time"
)

var upgrader = websocket.Upgrader {
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

var cdnPath string = "https://storage.googleapis.com/omsistuff-cdn/programs/autodl/repaints/"

const version = "0.0.1"

func doSelfUpdate() {
    v := semver.MustParse(version)
    latest, err := selfupdate.UpdateSelf(v, "omsistuff/assistant-d-installation")
    if err != nil {
        log.Println("Binary update failed:", err)
        return
    }
    if latest.Version.Equals(v) {
        // latest version is the same as current version. It means current binary is up to date.
        log.Println("Current binary is the latest version", version)
    } else {
        log.Println("Successfully updated to version", latest.Version)
        log.Println("Release note:\n", latest.ReleaseNotes)
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
        os.Exit(1)
    }

    fmt.Printf("Download saved to ./%v \n", resp.Filename)
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
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

    os.Exit(0)

}

func main() {
    fmt.Printf("Assistant d'installation version %v - (c) Omsistuff 2022\n", version)
    doSelfUpdate()
    fmt.Println("Starting local server on port 5300")
    http.HandleFunc("/", wsEndpoint)

    log.Fatal(http.ListenAndServe(":5300", nil))
}
