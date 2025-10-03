package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	filesvcv1 "github.com/suprt/test_proj/api/gen/filesvc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	serverAddr := flag.String("addr", "localhost:50051", "server address")
	cmd := flag.String("cmd", "list", "command: upload|download|list")
	file := flag.String("file", "", "filename for upload/download")
	out := flag.String("out", "", "output path for download")
	flag.Parse()

	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	cli := filesvcv1.NewFileServiceClient(conn)

	switch *cmd {
	case "upload":
		if *file == "" {
			log.Fatal("-file required")
		}
		if err := doUpload(cli, *file); err != nil {
			log.Fatalf("upload: %v", err)
		}
	case "download":
		if *file == "" {
			log.Fatal("-file required")
		}
		if err := doDownload(cli, *file, *out); err != nil {
			log.Fatalf("download: %v", err)
		}
	case "list":
		if err := doList(cli); err != nil {
			log.Fatalf("list: %v", err)
		}
	default:
		log.Fatalf("unknown cmd: %s", *cmd)
	}
}

func doUpload(cli filesvcv1.FileServiceClient, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	stream, err := cli.Upload(ctx)
	if err != nil {
		return err
	}
	// First chunk includes filename
	if err := stream.Send(&filesvcv1.UploadChunk{Filename: filepathBase(path)}); err != nil {
		return err
	}
	buf := make([]byte, 64*1024)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			if err := stream.Send(&filesvcv1.UploadChunk{Data: buf[:n]}); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	fmt.Printf("uploaded %s (%d bytes)\n", resp.GetFilename(), resp.GetSizeBytes())
	return nil
}

func doDownload(cli filesvcv1.FileServiceClient, name, outPath string) error {
	if outPath == "" {
		outPath = name
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	stream, err := cli.Download(ctx, &filesvcv1.DownloadRequest{Filename: name})
	if err != nil {
		return err
	}
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if _, werr := f.Write(chunk.GetData()); werr != nil {
			return werr
		}
	}
	fmt.Printf("downloaded to %s\n", outPath)
	return nil
}

func doList(cli filesvcv1.FileServiceClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := cli.ListFiles(ctx, &filesvcv1.ListFilesRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("Files (%d):\n", len(resp.GetFiles()))
	fmt.Printf("Filename | Created At | Updated At\n")
	fmt.Printf("---------|------------|------------\n")
	for _, fi := range resp.GetFiles() {
		fmt.Printf("%s | %s | %s\n", fi.GetFilename(), fi.GetCreatedAt(), fi.GetUpdatedAt())
	}
	return nil
}

func filepathBase(p string) string {
	// small local helper to avoid an import for one use
	i := len(p) - 1
	for i >= 0 && p[i] == '/' {
		i--
	}
	j := i
	for j >= 0 && p[j] != '/' {
		j--
	}
	return p[j+1 : i+1]
}
