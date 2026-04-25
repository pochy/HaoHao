package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

type FileObjectBody struct {
	PublicID         string    `json:"publicId" format:"uuid"`
	TenantID         int64     `json:"tenantId"`
	Purpose          string    `json:"purpose" example:"attachment"`
	AttachedToType   string    `json:"attachedToType,omitempty" example:"customer_signal"`
	AttachedToID     string    `json:"attachedToId,omitempty" format:"uuid"`
	OriginalFilename string    `json:"originalFilename"`
	ContentType      string    `json:"contentType" example:"text/plain"`
	ByteSize         int64     `json:"byteSize"`
	SHA256Hex        string    `json:"sha256Hex"`
	Status           string    `json:"status" example:"active"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time `json:"updatedAt" format:"date-time"`
}

type ListFilesInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	AttachedToType string      `query:"attachedToType"`
	AttachedToID   string      `query:"attachedToId"`
}

type FileListOutput struct {
	Body struct {
		Items []FileObjectBody `json:"items"`
	}
}

type DeleteFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
}

type DeleteFileOutput struct{}

func registerFileRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listFiles",
		Method:      http.MethodGet,
		Path:        "/api/v1/files",
		Summary:     "active tenant の file metadata を返す",
		Tags:        []string{"files"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListFilesInput) (*FileListOutput, error) {
		_, tenant, err := requireActiveTenantRole(ctx, deps, input.SessionCookie.Value, "", "", "file service")
		if err != nil {
			return nil, err
		}
		if input.AttachedToType == "" || input.AttachedToID == "" {
			return nil, huma.Error400BadRequest("attachedToType and attachedToId are required")
		}
		items, err := deps.FileService.ListForAttachment(ctx, tenant.ID, input.AttachedToType, input.AttachedToID)
		if err != nil {
			return nil, toFileHTTPError(err)
		}
		out := &FileListOutput{}
		out.Body.Items = make([]FileObjectBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toFileObjectBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteFile",
		Method:        http.MethodDelete,
		Path:          "/api/v1/files/{filePublicId}",
		Summary:       "active tenant の file を soft delete する",
		Tags:          []string{"files"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeleteFileInput) (*DeleteFileOutput, error) {
		current, tenant, err := requireActiveTenantRole(ctx, deps, input.SessionCookie.Value, input.CSRFToken, "", "file service")
		if err != nil {
			return nil, err
		}
		if err := deps.FileService.Delete(ctx, tenant.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toFileHTTPError(err)
		}
		return &DeleteFileOutput{}, nil
	})
}

func RegisterRawFileRoutes(router *gin.Engine, deps Dependencies, maxBytes int64) {
	if router == nil {
		return
	}
	router.POST("/api/v1/files", func(c *gin.Context) {
		current, tenant, ok := rawActiveTenant(c, deps, true)
		if !ok {
			return
		}
		if deps.FileService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"title": "file service is not configured"})
			return
		}
		if err := c.Request.ParseMultipartForm(maxBytes + 1024*1024); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"title": "invalid multipart form"})
			return
		}
		header, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"title": "file is required"})
			return
		}
		file, err := header.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"title": "file is invalid"})
			return
		}
		defer file.Close()
		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" {
			var sample [512]byte
			n, _ := file.Read(sample[:])
			contentType = http.DetectContentType(sample[:n])
			_, _ = file.Seek(0, io.SeekStart)
		}
		item, err := deps.FileService.Upload(c.Request.Context(), service.FileUploadInput{
			TenantID:         tenant.ID,
			UserID:           current.User.ID,
			Purpose:          c.PostForm("purpose"),
			AttachedToType:   c.PostForm("attachedToType"),
			AttachedToID:     c.PostForm("attachedToId"),
			OriginalFilename: header.Filename,
			ContentType:      contentType,
			Body:             file,
		}, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawFileError(c, err)
			return
		}
		c.JSON(http.StatusOK, toFileObjectBody(item))
	})

	router.GET("/api/v1/files/:filePublicId", func(c *gin.Context) {
		_, tenant, ok := rawActiveTenant(c, deps, false)
		if !ok {
			return
		}
		if deps.FileService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"title": "file service is not configured"})
			return
		}
		download, err := deps.FileService.Download(c.Request.Context(), tenant.ID, c.Param("filePublicId"))
		if err != nil {
			writeRawFileError(c, err)
			return
		}
		defer download.Body.Close()
		c.Header("Content-Type", download.File.ContentType)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", download.File.OriginalFilename))
		c.Header("X-Content-Type-Options", "nosniff")
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, download.Body)
	})
}

func rawActiveTenant(c *gin.Context, deps Dependencies, csrf bool) (service.CurrentSession, service.TenantAccess, bool) {
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"title": "missing or expired session"})
		return service.CurrentSession{}, service.TenantAccess{}, false
	}
	csrfToken := ""
	if csrf {
		csrfToken = c.GetHeader("X-CSRF-Token")
	}
	current, tenant, requireErr := requireActiveTenantRole(c.Request.Context(), deps, cookie, csrfToken, "", "file service")
	if requireErr != nil {
		var statusErr huma.StatusError
		if errors.As(requireErr, &statusErr) {
			c.JSON(statusErr.GetStatus(), gin.H{"title": statusErr.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"title": "internal server error"})
		}
		return service.CurrentSession{}, service.TenantAccess{}, false
	}
	return current, tenant, true
}

func writeRawFileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidFileInput):
		c.JSON(http.StatusBadRequest, gin.H{"title": "invalid file input"})
	case errors.Is(err, service.ErrFileQuotaExceeded):
		c.JSON(http.StatusConflict, gin.H{"title": "file quota exceeded"})
	case errors.Is(err, service.ErrFileNotFound):
		c.JSON(http.StatusNotFound, gin.H{"title": "file not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"title": "internal server error"})
	}
}

func toFileObjectBody(item service.FileObject) FileObjectBody {
	return FileObjectBody{
		PublicID:         item.PublicID,
		TenantID:         item.TenantID,
		Purpose:          item.Purpose,
		AttachedToType:   item.AttachedToType,
		AttachedToID:     item.AttachedToID,
		OriginalFilename: item.OriginalFilename,
		ContentType:      item.ContentType,
		ByteSize:         item.ByteSize,
		SHA256Hex:        item.SHA256Hex,
		Status:           item.Status,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toFileHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidFileInput):
		return huma.Error400BadRequest("invalid file input")
	case errors.Is(err, service.ErrFileQuotaExceeded):
		return huma.Error409Conflict("file quota exceeded")
	case errors.Is(err, service.ErrFileNotFound):
		return huma.Error404NotFound("file not found")
	default:
		return toHTTPError(err)
	}
}
