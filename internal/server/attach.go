package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
)

// Attachment limits. The API's own limit is not known; these keep a request well below what a
// phone sends with a few photos.
const (
	maxAttachments       = 5
	maxAttachmentBytes   = 10 << 20
	maxAttachmentsTotal  = 20 << 20
	attachmentHashPrefix = 12
)

// attachmentTypes are the sniffed content types an attachment may have: photos and PDFs.
var attachmentTypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true, "image/bmp": true, "application/pdf": true,
}

// heifExtensions are photo formats http.DetectContentType does not know.
var heifExtensions = map[string]string{".heic": "image/heic", ".heif": "image/heif"}

type attachmentIn struct {
	Path          string `json:"path,omitempty" jsonschema:"an absolute path of a file on the server's host, inside a directory BYTOVEKONTO_ATTACH_DIRS allows"`
	ContentBase64 string `json:"content_base64,omitempty" jsonschema:"the file's content in standard base64, instead of path"`
	Name          string `json:"name,omitempty" jsonschema:"the file name the management sees (required with content_base64; default the base name of path)"`
}

type attachmentView struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
}

// attachments reads and checks the attachments of a write: photos or PDFs, from a directory the
// configuration allows or inline, within the size limits.
func (s *Server) attachments(in []attachmentIn) ([]api.File, []attachmentView, error) {
	if len(in) > maxAttachments {
		return nil, nil, fmt.Errorf("at most %d attachments", maxAttachments)
	}
	files := make([]api.File, 0, len(in))
	views := make([]attachmentView, 0, len(in))
	total := 0
	for i, a := range in {
		data, name, err := s.readAttachment(a)
		if err != nil {
			return nil, nil, fmt.Errorf("attachment %d: %w", i+1, err)
		}
		total += len(data)
		if total > maxAttachmentsTotal {
			return nil, nil, fmt.Errorf("the attachments are larger than %d MiB together", maxAttachmentsTotal>>20)
		}
		typ, err := attachmentType(name, data)
		if err != nil {
			return nil, nil, fmt.Errorf("attachment %d (%s): %w", i+1, name, err)
		}
		sum := sha256.Sum256(data)
		files = append(files, api.File{Content: base64.StdEncoding.EncodeToString(data), FileName: name})
		views = append(views, attachmentView{Name: name, Type: typ, Size: len(data), SHA256: hex.EncodeToString(sum[:])[:attachmentHashPrefix]})
	}
	return files, views, nil
}

func (s *Server) readAttachment(a attachmentIn) ([]byte, string, error) {
	path, content := strings.TrimSpace(a.Path), strings.TrimSpace(a.ContentBase64)
	switch {
	case path != "" && content != "":
		return nil, "", errors.New("give path or content_base64, not both")
	case content != "":
		if strings.TrimSpace(a.Name) == "" {
			return nil, "", errors.New("name is required with content_base64")
		}
		if base64.StdEncoding.DecodedLen(len(content)) > maxAttachmentBytes+3 {
			return nil, "", fmt.Errorf("larger than %d MiB", maxAttachmentBytes>>20)
		}
		data, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, "", errors.New("content_base64 is not standard base64")
		}
		return checkSize(data, attachmentName(a.Name))
	case path != "":
		data, err := s.readAllowed(path)
		if err != nil {
			return nil, "", err
		}
		name := a.Name
		if strings.TrimSpace(name) == "" {
			name = filepath.Base(path)
		}
		return checkSize(data, attachmentName(name))
	}
	return nil, "", errors.New("path or content_base64 is required")
}

func checkSize(data []byte, name string) ([]byte, string, error) {
	switch {
	case len(data) == 0:
		return nil, "", errors.New("the file is empty")
	case len(data) > maxAttachmentBytes:
		return nil, "", fmt.Errorf("larger than %d MiB", maxAttachmentBytes>>20)
	}
	return data, name, nil
}

// readAllowed reads a file inside one of the attachment directories; symbolic links may not lead
// out of it.
func (s *Server) readAllowed(path string) ([]byte, error) {
	if len(s.opts.AttachDirs) == 0 {
		return nil, errors.New("attaching files by path is not enabled (BYTOVEKONTO_ATTACH_DIRS); send content_base64")
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("path must be absolute")
	}
	path = filepath.Clean(path)
	for _, dir := range s.opts.AttachDirs {
		rel, err := filepath.Rel(dir, path)
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		f, err := os.OpenInRoot(dir, rel)
		if err != nil {
			return nil, fmt.Errorf("cannot read the file: %w", errors.Unwrap(err))
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() {
			return nil, errors.New("not a regular file")
		}
		return io.ReadAll(io.LimitReader(f, maxAttachmentBytes+1))
	}
	return nil, errors.New("the file is not inside a directory BYTOVEKONTO_ATTACH_DIRS allows")
}

// attachmentName is the file name the management sees: a plain base name.
func attachmentName(name string) string {
	name = unsafeName.ReplaceAllString(filepath.Base(strings.TrimSpace(name)), "_")
	name = strings.Trim(name, ". ")
	if name == "" {
		return "attachment"
	}
	return name
}

func attachmentType(name string, data []byte) (string, error) {
	if t, ok := heifExtensions[strings.ToLower(filepath.Ext(name))]; ok {
		return t, nil
	}
	t := http.DetectContentType(data)
	if i := strings.IndexByte(t, ';'); i >= 0 {
		t = t[:i]
	}
	if !attachmentTypes[t] {
		return "", fmt.Errorf("only photos and PDFs can be attached, this is %s", t)
	}
	return t, nil
}
