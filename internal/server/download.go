package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
)

// maxDownloadBytes bounds one download.
const maxDownloadBytes = 100 << 20

var unsafeName = regexp.MustCompile(`[^\p{L}\p{N}._ -]+`)

// safeFileName keeps a file name from the API inside the download directory.
func safeFileName(id, name string) string {
	name = unsafeName.ReplaceAllString(filepath.Base(name), "_")
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "document"
	}
	return unsafeName.ReplaceAllString(id, "_") + "-" + name
}

func (s *Server) downloadDocument(ctx context.Context, _ *mcp.CallToolRequest, in documentIn) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, nil, errors.New("id is required")
	}
	ds, err := s.api.Documents(ctx)
	if err != nil {
		return nil, nil, err
	}
	var doc *api.Document
	for i := range ds {
		if ds[i].ID == id {
			doc = &ds[i]
			break
		}
	}
	if doc == nil {
		return nil, nil, fmt.Errorf("no document %q: list_documents lists them", id)
	}
	dl, err := s.api.DownloadDocument(ctx, *doc)
	if err != nil {
		return nil, nil, err
	}
	defer dl.Body.Close()
	data, err := io.ReadAll(io.LimitReader(dl.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("download: %w", err)
	}
	if len(data) > maxDownloadBytes {
		return nil, nil, fmt.Errorf("the file is larger than %d MiB", maxDownloadBytes>>20)
	}
	mimeType := dl.ContentType
	if mimeType == "" {
		mimeType = doc.FileMeta.ContentType
	}
	out := map[string]any{"document": viewDocument(*doc), "size": len(data), "content_type": mimeType}
	if s.opts.DownloadDir != "" {
		path, err := s.save(doc, data)
		if err != nil {
			return nil, nil, err
		}
		out["saved_to"] = path
	}
	res := &mcp.CallToolResult{}
	if int64(len(data)) <= s.opts.MaxInlineBytes {
		res.Content = append(res.Content, &mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
			URI: "bytovekonto://documents/" + id + "/" + doc.FileMeta.Filename, MIMEType: mimeType, Blob: data,
		}})
	} else {
		out["note"] = "the file is too large to return inline"
	}
	if s.opts.DownloadDir == "" && len(res.Content) == 0 {
		return nil, nil, errors.New("the file is too large to return inline and saving to disk is not configured")
	}
	// the SDK adds no text for a structured result when content is set: clients that read only
	// the content still get the path and the metadata
	b, err := json.Marshal(out)
	if err != nil {
		return nil, nil, fmt.Errorf("encode result: %w", err)
	}
	res.Content = append([]mcp.Content{&mcp.TextContent{Text: string(b)}}, res.Content...)
	return res, out, nil
}

func (s *Server) save(doc *api.Document, data []byte) (string, error) {
	if err := os.MkdirAll(s.opts.DownloadDir, 0o700); err != nil {
		return "", fmt.Errorf("download directory: %w", err)
	}
	path := filepath.Join(s.opts.DownloadDir, safeFileName(doc.ID, doc.FileMeta.Filename))
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) { //nolint:gosec // the name is sanitised and joined to the configured directory
		return path, nil
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("save: %w", err)
	}
	return path, nil
}
