package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	filesvcv1 "github.com/suprt/test_proj/api/gen/filesvc/v1"
	"github.com/suprt/test_proj/internal/limiter"
	"github.com/suprt/test_proj/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FileStore defines the interface for file storage operations
type FileStore interface {
	SaveStream(ctx context.Context, filename string, write func(w io.Writer) (int64, error)) (storage.Metadata, error)
	Open(ctx context.Context, filename string) (io.ReadCloser, storage.Metadata, error)
	List(ctx context.Context) ([]storage.Metadata, error)
}

type FileServiceServer struct {
	filesvcv1.UnimplementedFileServiceServer
	store FileStore
}

func NewFileServiceServer(store FileStore) *FileServiceServer {
	return &FileServiceServer{store: store}
}

func (s *FileServiceServer) Upload(stream filesvcv1.FileService_UploadServer) error {
	// Получаем первый чанк чтобы узнать filename
	firstChunk, err := stream.Recv()
	if err != nil {
		return err
	}
	filename := firstChunk.GetFilename()
	if filename == "" {
		return fmt.Errorf("filename is required in first chunk")
	}

	// Создаем write функцию для обработки всех чанков
	write := func(w io.Writer) (int64, error) {
		var total int64

		// Записываем данные из первого чанка
		if data := firstChunk.GetData(); len(data) > 0 {
			n, err := w.Write(data)
			total += int64(n)
			if err != nil {
				return total, err
			}
		}

		// Обрабатываем остальные чанки
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return total, nil
			}
			if err != nil {
				return total, err
			}
			if data := chunk.GetData(); len(data) > 0 {
				n, werr := w.Write(data)
				total += int64(n)
				if werr != nil {
					return total, werr
				}
			}
		}
	}

	meta, err := s.store.SaveStream(stream.Context(), filename, write)
	if err != nil {
		return err
	}
	return stream.SendAndClose(&filesvcv1.UploadResponse{Filename: meta.Filename, SizeBytes: meta.SizeBytes})
}

func (s *FileServiceServer) Download(req *filesvcv1.DownloadRequest, stream filesvcv1.FileService_DownloadServer) error {
	rc, _, err := s.store.Open(stream.Context(), req.GetFilename())
	if err != nil {
		return err
	}
	defer rc.Close()

	buf := make([]byte, 64*1024)
	for {
		n, rerr := rc.Read(buf)
		if n > 0 {
			if err := stream.Send(&filesvcv1.DownloadChunk{Data: buf[:n]}); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

func (s *FileServiceServer) ListFiles(ctx context.Context, _ *filesvcv1.ListFilesRequest) (*filesvcv1.ListFilesResponse, error) {
	items, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]*filesvcv1.FileInfo, 0, len(items))
	for _, m := range items {
		out = append(out, &filesvcv1.FileInfo{
			Filename:  m.Filename,
			CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt: m.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	return &filesvcv1.ListFilesResponse{Files: out}, nil
}

// Run starts the gRPC server on addr with provided data directory and concurrency limits.
func Run(addr, dataDir string) error {
	store, err := storage.NewFilesystemStore(dataDir)
	if err != nil {
		return err
	}
	lim := limiter.New(10, 100)

	// Create server with FileStore interface
	server := NewFileServiceServer(store)

	gs := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()),
		grpc.ChainUnaryInterceptor(lim.UnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(lim.StreamServerInterceptor()),
	)
	filesvcv1.RegisterFileServiceServer(gs, server)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return gs.Serve(ln)
}

// Server relies on client-provided deadlines/timeouts; no server-side default timeout applied.
