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

type DriveFileOutput struct {
	Body DriveFileBody
}

type GetDriveFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	FilePublicID  string      `path:"filePublicId"`
}

type UpdateDriveFileBody struct {
	OriginalFilename     *string `json:"originalFilename,omitempty" maxLength:"255"`
	ParentFolderPublicID *string `json:"parentFolderPublicId,omitempty"`
}

type UpdateDriveFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          UpdateDriveFileBody
}

type DeleteDriveFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
}

type UpdateDriveFileInheritanceInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          DriveInheritanceBody
}

func registerDriveFileRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getDriveFile",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/files/{filePublicId}",
		Summary:     "Drive file metadata を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveFileInput) (*DriveFileOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		file, err := deps.DriveService.GetFile(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFileOutput{Body: toDriveFileBody(file)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveFile",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/files/{filePublicId}",
		Summary:     "Drive file metadata を更新する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateDriveFileInput) (*DriveFileOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		file, err := deps.DriveService.UpdateFile(ctx, service.DriveUpdateFileInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			FilePublicID:         input.FilePublicID,
			Filename:             input.Body.OriginalFilename,
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFileOutput{Body: toDriveFileBody(file)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveFile",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/files/{filePublicId}",
		Summary:       "Drive file を削除する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveFileInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.DeleteFile(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveFileInheritance",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/files/{filePublicId}/inheritance",
		Summary:     "Drive file inheritance を更新する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateDriveFileInheritanceInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		ref := service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID}
		if input.Body.Enabled {
			err = deps.DriveService.ResumeInheritance(ctx, tenant.ID, current.User.ID, ref, sessionAuditContext(ctx, current, &tenant.ID))
		} else {
			err = deps.DriveService.StopInheritance(ctx, tenant.ID, current.User.ID, ref, sessionAuditContext(ctx, current, &tenant.ID))
		}
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})
}

func RegisterRawDriveRoutes(router *gin.Engine, deps Dependencies, maxBytes int64) {
	if router == nil {
		return
	}
	router.POST("/api/v1/drive/files", func(c *gin.Context) {
		current, tenant, ok := rawDriveActiveTenant(c, deps, true)
		if !ok {
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
		item, err := deps.DriveService.UploadFile(c.Request.Context(), service.DriveUploadFileInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			WorkspacePublicID:    c.PostForm("workspacePublicId"),
			ParentFolderPublicID: c.PostForm("parentFolderPublicId"),
			Filename:             header.Filename,
			ContentType:          contentType,
			Body:                 file,
		}, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		c.JSON(http.StatusOK, toDriveFileBody(item))
	})

	router.GET("/api/v1/drive/files/:filePublicId/content", func(c *gin.Context) {
		current, tenant, ok := rawDriveActiveTenant(c, deps, false)
		if !ok {
			return
		}
		download, err := deps.DriveService.DownloadFile(c.Request.Context(), tenant.ID, current.User.ID, c.Param("filePublicId"), sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		defer download.Body.Close()
		writeDriveDownload(c, download)
	})

	router.PUT("/api/v1/drive/files/:filePublicId/content", func(c *gin.Context) {
		current, tenant, ok := rawDriveActiveTenant(c, deps, true)
		if !ok {
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
			contentType = "application/octet-stream"
		}
		item, err := deps.DriveService.OverwriteFile(c.Request.Context(), service.DriveOverwriteFileInput{
			TenantID:     tenant.ID,
			ActorUserID:  current.User.ID,
			FilePublicID: c.Param("filePublicId"),
			Filename:     header.Filename,
			ContentType:  contentType,
			Body:         file,
		}, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		c.JSON(http.StatusOK, toDriveFileBody(item))
	})

	router.GET("/api/public/drive/share-links/:token/content", func(c *gin.Context) {
		if deps.DriveService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"title": "drive service is not configured"})
			return
		}
		verificationCookie, _ := c.Cookie(service.DriveShareLinkPasswordCookieName)
		download, err := deps.DriveService.PublicShareLinkContentWithVerification(c.Request.Context(), c.Param("token"), verificationCookie)
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		defer download.Body.Close()
		writeDriveDownload(c, download)
	})

	router.POST("/api/public/drive/share-links/:token/password", func(c *gin.Context) {
		if deps.DriveService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"title": "drive service is not configured"})
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"title": "invalid drive input"})
			return
		}
		verification, err := deps.DriveService.VerifyPublicShareLinkPassword(c.Request.Context(), c.Param("token"), body.Password, c.ClientIP())
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		maxAge := int(time.Until(verification.ExpiresAt).Seconds())
		if maxAge < 0 {
			maxAge = 0
		}
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(verification.CookieName, verification.CookieValue, maxAge, "/api/public/drive/share-links/"+c.Param("token"), "", false, true)
		c.JSON(http.StatusOK, gin.H{"verified": true, "expiresAt": verification.ExpiresAt})
	})

	router.PUT("/api/public/drive/share-links/:token/content", func(c *gin.Context) {
		if deps.DriveService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"title": "drive service is not configured"})
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
			contentType = "application/octet-stream"
		}
		verificationCookie, _ := c.Cookie(service.DriveShareLinkPasswordCookieName)
		item, err := deps.DriveService.PublicShareLinkOverwriteContentWithVerification(c.Request.Context(), service.DrivePublicEditorOverwriteInput{
			Token:              c.Param("token"),
			VerificationCookie: verificationCookie,
			Filename:           header.Filename,
			ContentType:        contentType,
			Body:               file,
		})
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		c.JSON(http.StatusOK, toDriveFileBody(item))
	})

	router.GET("/api/v1/admin/tenants/:tenantSlug/drive/files/:filePublicId/content", func(c *gin.Context) {
		if deps.DriveService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"title": "drive service is not configured"})
			return
		}
		cookie, err := c.Cookie(auth.SessionCookieName)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"title": "missing or expired session"})
			return
		}
		current, tenant, requireErr := requireAdminTenantID(c.Request.Context(), deps, cookie, "", c.Param("tenantSlug"))
		if requireErr != nil {
			var statusErr huma.StatusError
			if errors.As(requireErr, &statusErr) {
				c.JSON(statusErr.GetStatus(), gin.H{"title": statusErr.Error()})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"title": "internal server error"})
			}
			return
		}
		download, err := deps.DriveService.AdminDriveFileContent(c.Request.Context(), tenant.ID, current.User.ID, c.Param("filePublicId"), sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		defer download.Body.Close()
		writeDriveDownload(c, download)
	})

	router.GET("/api/external/v1/drive/files/:fileId/content", func(c *gin.Context) {
		authCtx, tenant, ok := rawExternalDriveUser(c, "drive:read")
		if !ok {
			return
		}
		download, err := deps.DriveService.DownloadFile(c.Request.Context(), tenant.ID, authCtx.User.ID, c.Param("fileId"), externalDriveAuditContext(authCtx, tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		defer download.Body.Close()
		writeDriveDownload(c, download)
	})

	router.POST("/api/external/v1/drive/files", func(c *gin.Context) {
		authCtx, tenant, ok := rawExternalDriveUser(c, "drive:write")
		if !ok {
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
			contentType = "application/octet-stream"
		}
		item, err := deps.DriveService.UploadFile(c.Request.Context(), service.DriveUploadFileInput{
			TenantID:             tenant.ID,
			ActorUserID:          authCtx.User.ID,
			WorkspacePublicID:    c.PostForm("workspacePublicId"),
			ParentFolderPublicID: c.PostForm("parentFolderPublicId"),
			Filename:             header.Filename,
			ContentType:          contentType,
			Body:                 file,
		}, externalDriveAuditContext(authCtx, tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		c.JSON(http.StatusOK, toDriveFileBody(item))
	})
}

func rawExternalDriveUser(c *gin.Context, scope string) (service.AuthContext, service.TenantAccess, bool) {
	authCtx, tenant, err := requireExternalDriveUser(c.Request.Context(), scope)
	if err != nil {
		var statusErr huma.StatusError
		if errors.As(err, &statusErr) {
			c.JSON(statusErr.GetStatus(), gin.H{"title": statusErr.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"title": "internal server error"})
		}
		return service.AuthContext{}, service.TenantAccess{}, false
	}
	return authCtx, tenant, true
}

func rawDriveActiveTenant(c *gin.Context, deps Dependencies, csrf bool) (service.CurrentSession, service.TenantAccess, bool) {
	if deps.DriveService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"title": "drive service is not configured"})
		return service.CurrentSession{}, service.TenantAccess{}, false
	}
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"title": "missing or expired session"})
		return service.CurrentSession{}, service.TenantAccess{}, false
	}
	csrfToken := ""
	if csrf {
		csrfToken = c.GetHeader("X-CSRF-Token")
	}
	current, tenant, requireErr := requireDriveTenant(c.Request.Context(), deps, cookie, csrfToken)
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

func writeRawDriveError(c *gin.Context, err error) {
	c.JSON(driveStatusCode(err), gin.H{"title": driveErrorTitle(err)})
}

func driveErrorTitle(err error) string {
	switch driveStatusCode(err) {
	case http.StatusBadRequest:
		return "invalid drive input"
	case http.StatusServiceUnavailable:
		return "drive authorization unavailable"
	case http.StatusForbidden:
		return "drive permission denied"
	case http.StatusConflict:
		if errors.Is(err, service.ErrDriveLocked) {
			return "drive resource is locked"
		}
		return "file quota exceeded"
	case http.StatusNotFound:
		return "drive resource not found"
	default:
		return "internal server error"
	}
}

func writeDriveDownload(c *gin.Context, download service.DriveFileDownload) {
	c.Header("Content-Type", download.File.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", download.File.OriginalFilename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, download.Body)
}
