package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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
	OriginalFilename     *string   `json:"originalFilename,omitempty" maxLength:"255"`
	Description          *string   `json:"description,omitempty" maxLength:"4000"`
	Tags                 *[]string `json:"tags,omitempty"`
	ParentFolderPublicID *string   `json:"parentFolderPublicId,omitempty"`
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
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveFileInput) (*DriveFileOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		file, err := deps.DriveService.GetFile(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveFileOutput{Body: toDriveFileBody(file)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveFile",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/files/{filePublicId}",
		Summary:     "Drive file metadata を更新する",
		Tags:        []string{DocTagDriveFilesFolders},
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
			Description:          input.Body.Description,
			Tags:                 input.Body.Tags,
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveFileOutput{Body: toDriveFileBody(file)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveFile",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/files/{filePublicId}",
		Summary:       "Drive file を削除する",
		Tags:          []string{DocTagDriveFilesFolders},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveFileInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.DeleteFile(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveFileInheritance",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/files/{filePublicId}/inheritance",
		Summary:     "Drive file inheritance を更新する",
		Tags:        []string{DocTagDriveSharingPermissions},
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
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})
}

func RegisterRawDriveRoutes(router *gin.Engine, deps Dependencies, maxBytes, datasetMaxBytes int64) {
	if router == nil {
		return
	}
	router.POST("/api/v1/drive/files", func(c *gin.Context) {
		current, tenant, ok := rawDriveActiveTenant(c, deps, true)
		if !ok {
			return
		}
		reader, err := c.Request.MultipartReader()
		if err != nil {
			writeRawDriveMultipartReaderError(c, err, maxRawDriveUploadBytes(maxBytes, datasetMaxBytes))
			return
		}
		var workspacePublicID string
		var parentFolderPublicID string
		var item service.DriveFile
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				writeRawDriveMultipartReaderError(c, err, maxRawDriveUploadBytes(maxBytes, datasetMaxBytes))
				return
			}
			switch part.FormName() {
			case "workspacePublicId":
				workspacePublicID = readRawDriveMultipartField(part, 128)
				_ = part.Close()
			case "parentFolderPublicId":
				parentFolderPublicID = readRawDriveMultipartField(part, 128)
				_ = part.Close()
			case "file":
				if item.ID > 0 {
					_ = part.Close()
					writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorInvalidMultipart, "Only one file is allowed."))
					return
				}
				body := bufio.NewReader(part)
				filename := filepath.Base(strings.TrimSpace(part.FileName()))
				contentType := rawDriveUploadContentType(filename, strings.TrimSpace(part.Header.Get("Content-Type")), body)
				uploadLimit := rawDriveUploadMaxBytes(filename, contentType, maxBytes, datasetMaxBytes)
				uploaded, err := deps.DriveService.UploadFile(c.Request.Context(), service.DriveUploadFileInput{
					TenantID:             tenant.ID,
					ActorUserID:          current.User.ID,
					WorkspacePublicID:    workspacePublicID,
					ParentFolderPublicID: parentFolderPublicID,
					Filename:             filename,
					ContentType:          contentType,
					Body:                 body,
					MaxBytes:             uploadLimit,
				}, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
				_ = part.Close()
				if err != nil {
					writeRawDriveError(c, err)
					return
				}
				item = uploaded
			default:
				_ = part.Close()
			}
		}
		if item.ID <= 0 {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorFileRequired, "File is required."))
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

	router.GET("/api/v1/drive/files/:filePublicId/preview", func(c *gin.Context) {
		current, tenant, ok := rawDriveActiveTenant(c, deps, false)
		if !ok {
			return
		}
		download, err := deps.DriveService.PreviewFile(c.Request.Context(), tenant.ID, current.User.ID, c.Param("filePublicId"), false, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		defer download.Body.Close()
		writeDriveInline(c, download)
	})

	router.GET("/api/v1/drive/files/:filePublicId/thumbnail", func(c *gin.Context) {
		current, tenant, ok := rawDriveActiveTenant(c, deps, false)
		if !ok {
			return
		}
		download, err := deps.DriveService.PreviewFile(c.Request.Context(), tenant.ID, current.User.ID, c.Param("filePublicId"), true, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawDriveError(c, err)
			return
		}
		defer download.Body.Close()
		writeDriveInline(c, download)
	})

	router.PUT("/api/v1/drive/files/:filePublicId/content", func(c *gin.Context) {
		current, tenant, ok := rawDriveActiveTenant(c, deps, true)
		if !ok {
			return
		}
		reader, err := c.Request.MultipartReader()
		if err != nil {
			writeRawDriveMultipartReaderError(c, err, maxRawDriveUploadBytes(maxBytes, datasetMaxBytes))
			return
		}
		var item service.DriveFile
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				writeRawDriveMultipartReaderError(c, err, maxRawDriveUploadBytes(maxBytes, datasetMaxBytes))
				return
			}
			if part.FormName() != "file" {
				_ = part.Close()
				continue
			}
			if item.ID > 0 {
				_ = part.Close()
				writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorInvalidMultipart, "Only one file is allowed."))
				return
			}
			body := bufio.NewReader(part)
			filename := filepath.Base(strings.TrimSpace(part.FileName()))
			contentType := rawDriveUploadContentType(filename, strings.TrimSpace(part.Header.Get("Content-Type")), body)
			uploadLimit := rawDriveUploadMaxBytes(filename, contentType, maxBytes, datasetMaxBytes)
			updated, err := deps.DriveService.OverwriteFile(c.Request.Context(), service.DriveOverwriteFileInput{
				TenantID:     tenant.ID,
				ActorUserID:  current.User.ID,
				FilePublicID: c.Param("filePublicId"),
				Filename:     filename,
				ContentType:  contentType,
				Body:         body,
				MaxBytes:     uploadLimit,
			}, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
			_ = part.Close()
			if err != nil {
				writeRawDriveError(c, err)
				return
			}
			item = updated
		}
		if item.ID <= 0 {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorFileRequired, "File is required."))
			return
		}
		c.JSON(http.StatusOK, toDriveFileBody(item))
	})

	router.GET("/api/public/drive/share-links/:token/content", func(c *gin.Context) {
		if deps.DriveService == nil {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusServiceUnavailable, "drive.service_unavailable", "Drive service is not configured."))
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
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusServiceUnavailable, "drive.service_unavailable", "Drive service is not configured."))
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, "drive.invalid_input", "Drive request body is invalid."))
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
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusServiceUnavailable, "drive.service_unavailable", "Drive service is not configured."))
			return
		}
		if err := c.Request.ParseMultipartForm(maxBytes + 1024*1024); err != nil {
			if isRequestBodyTooLargeError(err) {
				writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusRequestEntityTooLarge, service.DriveErrorFileTooLarge, fmt.Sprintf("File exceeds the Drive upload limit of %s.", formatUploadLimitLabel(maxBytes))))
				return
			}
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorInvalidMultipart, "Multipart form is invalid or exceeds the request size limit."))
			return
		}
		header, err := c.FormFile("file")
		if err != nil {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorFileRequired, "File is required."))
			return
		}
		file, err := header.Open()
		if err != nil {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorInvalidMultipart, "File body is invalid."))
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
			if isRequestBodyTooLargeError(err) {
				writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusRequestEntityTooLarge, service.DriveErrorFileTooLarge, fmt.Sprintf("File exceeds the Drive upload limit of %s.", formatUploadLimitLabel(maxBytes))))
				return
			}
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorInvalidMultipart, "Multipart form is invalid or exceeds the request size limit."))
			return
		}
		header, err := c.FormFile("file")
		if err != nil {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorFileRequired, "File is required."))
			return
		}
		file, err := header.Open()
		if err != nil {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorInvalidMultipart, "File body is invalid."))
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

func readRawDriveMultipartField(part io.Reader, maxLen int64) string {
	if maxLen <= 0 {
		maxLen = 512
	}
	value, _ := io.ReadAll(io.LimitReader(part, maxLen))
	return strings.TrimSpace(string(value))
}

func rawDriveUploadContentType(filename, contentType string, body *bufio.Reader) string {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if (contentType == "" || contentType == "application/octet-stream") && body != nil {
		sample, _ := body.Peek(512)
		if len(sample) > 0 {
			contentType = strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(sample), ";")[0]))
		}
	}
	if rawDriveLooksLikeCSV(filename, contentType) {
		return "text/csv"
	}
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func rawDriveUploadMaxBytes(filename, contentType string, fileMaxBytes, datasetMaxBytes int64) int64 {
	if rawDriveLooksLikeCSV(filename, contentType) && datasetMaxBytes > fileMaxBytes {
		return datasetMaxBytes
	}
	return 0
}

func maxRawDriveUploadBytes(fileMaxBytes, datasetMaxBytes int64) int64 {
	if datasetMaxBytes > fileMaxBytes {
		return datasetMaxBytes
	}
	return fileMaxBytes
}

func rawDriveLooksLikeCSV(filename, contentType string) bool {
	if strings.EqualFold(filepath.Ext(strings.TrimSpace(filename)), ".csv") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "text/csv", "application/csv", "application/vnd.ms-excel":
		return true
	default:
		return false
	}
}

func writeRawDriveMultipartReaderError(c *gin.Context, err error, maxBytes int64) {
	if isRequestBodyTooLargeError(err) {
		writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusRequestEntityTooLarge, service.DriveErrorFileTooLarge, fmt.Sprintf("File exceeds the Drive upload limit of %s.", formatUploadLimitLabel(maxBytes))))
		return
	}
	writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusBadRequest, service.DriveErrorInvalidMultipart, "Multipart form is invalid or exceeds the request size limit."))
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
		writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusServiceUnavailable, "drive.service_unavailable", "Drive service is not configured."))
		return service.CurrentSession{}, service.TenantAccess{}, false
	}
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err != nil {
		writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusUnauthorized, "drive.session_required", "Missing or expired session."))
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
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), statusErr.GetStatus(), "drive.tenant_access_denied", statusErr.Error()))
		} else {
			writeRawDriveProblem(c, driveProblemFromInput(c.Request.Context(), http.StatusInternalServerError, "drive.internal", "Drive request failed. Contact support with the request ID."))
		}
		return service.CurrentSession{}, service.TenantAccess{}, false
	}
	return current, tenant, true
}

func writeRawDriveError(c *gin.Context, err error) {
	writeRawDriveProblem(c, driveProblemFromError(c.Request.Context(), err))
}

func writeRawDriveProblem(c *gin.Context, problem driveProblem) {
	c.Set("error_type", problem.Type)
	c.Set("error_code", problem.Code)
	c.Set("error_detail", problem.Detail)
	c.JSON(problem.Status, problem)
}

func writeDriveDownload(c *gin.Context, download service.DriveFileDownload) {
	c.Header("Content-Type", download.File.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", download.File.OriginalFilename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, download.Body)
}

func writeDriveInline(c *gin.Context, download service.DriveFileDownload) {
	c.Header("Content-Type", download.File.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", download.File.OriginalFilename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, download.Body)
}

func isRequestBodyTooLargeError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "request body too large")
}

func formatUploadLimitLabel(maxBytes int64) string {
	if maxBytes <= 0 {
		return "the configured limit"
	}
	const mb = int64(1024 * 1024)
	if maxBytes%mb == 0 {
		return fmt.Sprintf("%d MB", maxBytes/mb)
	}
	return fmt.Sprintf("%.1f MB", float64(maxBytes)/float64(mb))
}
