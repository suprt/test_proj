package main

import (
    "flag"
    "log"

    "github.com/suprt/test_proj/internal/server"
)

func main() {
    addr := flag.String("addr", ":50051", "listen address")
    data := flag.String("data", "./data", "directory to store files")
    flag.Parse()
    if err := server.Run(*addr, *data); err != nil {
        log.Fatalf("server error: %v", err)
    }
}
