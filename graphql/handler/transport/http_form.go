package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"strings"

	"github.com/RobertoOrtis/fastgql/graphql"
	"github.com/valyala/fasthttp"
)

// MultipartForm the Multipart request spec https://github.com/jaydenseric/graphql-multipart-request-spec
type MultipartForm struct {
	// MaxUploadSize sets the maximum number of bytes used to parse a request body
	// as multipart/form-data.
	MaxUploadSize int64

	// MaxMemory defines the maximum number of bytes used to parse a request body
	// as multipart/form-data in memory, with the remainder stored on disk in
	// temporary files.
	MaxMemory int64
}

var _ graphql.Transport = MultipartForm{}

func (f MultipartForm) Supports(ctx *fasthttp.RequestCtx) bool {
	if string(ctx.Request.Header.Peek("Upgrade")) != "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(string(ctx.Request.Header.ContentType()))
	if err != nil {
		return false
	}

	return string(ctx.Method()) == "POST" && mediaType == "multipart/form-data"
}

func (f MultipartForm) maxUploadSize() int64 {
	if f.MaxUploadSize == 0 {
		return 32 << 20
	}
	return f.MaxUploadSize
}

func (f MultipartForm) maxMemory() int64 {
	if f.MaxMemory == 0 {
		return 32 << 20
	}
	return f.MaxMemory
}

func (f MultipartForm) Do(ctx *fasthttp.RequestCtx, exec graphql.GraphExecutor) {
	ctx.Response.Header.SetContentType("application/json")

	start := graphql.Now()

	var err error
	if int64(ctx.Request.Header.ContentLength()) > f.maxUploadSize() {
		writeJsonError(ctx, "failed to parse multipart form, request body too large")
		return
	}

	if _, err = ctx.MultipartForm(); err != nil {
		ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
		if strings.Contains(err.Error(), "request body too large") {
			writeJsonError(ctx, "failed to parse multipart form, request body too large")
			return
		}
		writeJsonError(ctx, "failed to parse multipart form")
		return
	}

	var params graphql.RawParams

	if err = jsonDecode(bytes.NewReader(ctx.FormValue("operations")), &params); err != nil {
		ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
		writeJsonError(ctx, "operations form field could not be decoded")
		return
	}

	var uploadsMap = map[string][]string{}
	if err = json.Unmarshal(ctx.FormValue("map"), &uploadsMap); err != nil {
		ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
		writeJsonError(ctx, "map form field could not be decoded")
		return
	}

	var upload graphql.Upload
	for key, paths := range uploadsMap {
		if len(paths) == 0 {
			ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
			writeJsonErrorf(ctx, "invalid empty operations paths list for key %s", key)
			return
		}

		header, err := ctx.FormFile(key)
		if err != nil {
			ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
			writeJsonErrorf(ctx, "failed to get key %s from form", key)
			return
		}
		file, err := header.Open()
		if err != nil {
			ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
			writeJsonErrorf(ctx, "failed to get key %s from form", key)
			return
		}
		defer file.Close()

		if len(paths) == 1 {
			upload = graphql.Upload{
				File:        file,
				Size:        header.Size,
				Filename:    header.Filename,
				ContentType: header.Header.Get("Content-Type"),
			}

			if err := params.AddUpload(upload, key, paths[0]); err != nil {
				ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
				writeJsonGraphqlError(ctx, err)
				return
			}
		} else {
			if int64(ctx.Request.Header.ContentLength()) < f.maxMemory() {
				fileBytes, err := ioutil.ReadAll(file)
				if err != nil {
					ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
					writeJsonErrorf(ctx, "failed to read file for key %s", key)
					return
				}
				for _, path := range paths {
					upload = graphql.Upload{
						File:        &bytesReader{s: &fileBytes, i: 0, prevRune: -1},
						Size:        header.Size,
						Filename:    header.Filename,
						ContentType: header.Header.Get("Content-Type"),
					}

					if err := params.AddUpload(upload, key, path); err != nil {
						ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
						writeJsonGraphqlError(ctx, err)
						return
					}
				}
			} else {
				tmpFile, err := ioutil.TempFile(os.TempDir(), "gqlgen-")
				if err != nil {
					ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
					writeJsonErrorf(ctx, "failed to create temp file for key %s", key)
					return
				}
				tmpName := tmpFile.Name()
				defer func() {
					_ = os.Remove(tmpName)
				}()
				_, err = io.Copy(tmpFile, file)
				if err != nil {
					ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
					if err := tmpFile.Close(); err != nil {
						writeJsonErrorf(ctx, "failed to copy to temp file and close temp file for key %s", key)
						return
					}
					writeJsonErrorf(ctx, "failed to copy to temp file for key %s", key)
					return
				}
				if err := tmpFile.Close(); err != nil {
					ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
					writeJsonErrorf(ctx, "failed to close temp file for key %s", key)
					return
				}
				for _, path := range paths {
					pathTmpFile, err := os.Open(tmpName)
					if err != nil {
						ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
						writeJsonErrorf(ctx, "failed to open temp file for key %s", key)
						return
					}
					defer pathTmpFile.Close()
					upload = graphql.Upload{
						File:        pathTmpFile,
						Size:        header.Size,
						Filename:    header.Filename,
						ContentType: header.Header.Get("Content-Type"),
					}

					if err := params.AddUpload(upload, key, path); err != nil {
						ctx.Response.Header.SetStatusCode(fasthttp.StatusUnprocessableEntity)
						writeJsonGraphqlError(ctx, err)
						return
					}
				}
			}
		}
	}

	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, gerr := exec.CreateOperationContext(ctx, &params)
	if gerr != nil {
		resp := exec.DispatchError(graphql.WithOperationContext(ctx, rc), gerr)
		ctx.Response.Header.SetStatusCode(statusFor(gerr))
		writeJson(ctx, resp)
		return
	}
	responses, c := exec.DispatchOperation(ctx, rc)
	writeJson(ctx, responses(c))
}
