package api

import (
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

const (
	docsDemoTenantSlug = "acme"
	docsDemoUUID       = "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"
	docsDemoEmail      = "demo@example.com"
	docsDemoTime       = "2026-01-15T09:30:00Z"
)

type docsExampleMode int

const (
	docsExampleRequest docsExampleMode = iota
	docsExampleResponse
)

type operationDoc struct {
	Description     string
	RequestExample  any
	ResponseExample any
}

var openAPIOperationDocs = map[string]operationDoc{
	"login": {
		Description: "local password login が有効な環境で、email/password を検証して Cookie session を発行します。成功時は `SESSION_ID` Cookie が返り、以後の browser API はその Cookie と必要に応じて `X-CSRF-Token` を送ります。",
		RequestExample: map[string]any{
			"email":    docsDemoEmail,
			"password": "password",
		},
	},
	"selectTenant": {
		Description: "ログイン中の session に active tenant を設定します。tenant-scoped API はここで選択した tenant を既定の操作対象として扱います。",
		RequestExample: map[string]any{
			"tenantSlug": docsDemoTenantSlug,
		},
	},
	"createCustomerSignal": {
		Description: "active tenant に customer signal を作成します。重複送信を避けたい場合は `Idempotency-Key` を指定してください。",
		RequestExample: map[string]any{
			"customerName": "Acme",
			"title":        "Export CSV from reports",
			"body":         "Customer asked for monthly report export.",
			"source":       "support",
			"priority":     "medium",
			"status":       "new",
		},
	},
	"createDriveFolder": {
		Description: "Drive に folder を作成します。`parentFolderPublicId` を省略すると root 配下に作成し、`workspacePublicId` を指定すると workspace 配下で管理します。",
		RequestExample: map[string]any{
			"name":                 "Project Alpha",
			"description":          "Quarterly planning documents.",
			"parentFolderPublicId": docsDemoUUID,
			"workspacePublicId":    docsDemoUUID,
			"tags":                 []string{"planning", "finance"},
		},
	},
	"updateDriveFile": {
		Description: "Drive file の filename、description、tags、parent folder を更新します。ファイル本体の差し替えではなく metadata と配置の更新です。",
		RequestExample: map[string]any{
			"originalFilename":       "roadmap.md",
			"description":            "Updated product roadmap.",
			"tags":                   []string{"roadmap", "planning"},
			"parentFolderPublicId":   docsDemoUUID,
			"metadataVersionComment": "Moved into planning folder.",
		},
	},
	"createDriveFileShareLink": {
		Description: "Drive file に public share link を作成します。password や expiry を設定すると、link access の安全性を高められます。",
		RequestExample: map[string]any{
			"role":        "viewer",
			"canDownload": true,
			"expiresAt":   "2026-02-01T00:00:00Z",
			"password":    "share-demo-passphrase",
		},
	},
	"createWebhook": {
		Description: "tenant admin API で webhook endpoint を登録します。作成後の delivery には署名 header が付き、secret は rotate API で更新できます。",
		RequestExample: map[string]any{
			"name":       "Customer signal sink",
			"url":        "https://example.com/webhooks/haohao",
			"eventTypes": []string{"customer_signal.created"},
			"active":     true,
		},
	},
	"createTenantInvitation": {
		Description: "tenant に user を招待します。email と role codes を指定すると、招待 URL を含む invitation record が作成されます。",
		RequestExample: map[string]any{
			"email":     docsDemoEmail,
			"roleCodes": []string{"todo_user", "drive_user"},
		},
	},
	"createDataset": {
		Description: "Drive CSV file を dataset として登録します。取り込み対象の file は active tenant の Drive 内に存在し、dataset 作成権限が必要です。",
	},
	"createDatasetQueryJob": {
		Description: "dataset に対する SQL query job を作成します。query は tenant-scoped work table に限定して実行されます。",
		RequestExample: map[string]any{
			"sql": "select customer_name, count(*) as signal_count from customer_signals group by customer_name",
		},
	},
	"createDriveOCRJob": {
		Description: "Drive file の OCR job を作成します。画像/PDF から抽出された text は OCR result API で取得します。",
	},
	"createDriveProductExtractionJob": {
		Description: "OCR 結果をもとに product extraction job を作成します。抽出結果は product extraction list API で確認します。",
	},
	"createDriveAIJob": {
		Description: "Drive file に対する AI processing job を作成します。summary や classification のような後続取得 API と組み合わせて使います。",
		RequestExample: map[string]any{
			"jobType": "summary",
		},
	},
	"getExternalMe": {
		Description: "Bearer token で認証された external principal と tenant-aware context を返します。外部連携クライアントの認証確認に使います。",
	},
}

func OpenAPIInfoDescription() string {
	return strings.TrimSpace(
		"HaoHao API は browser session API、tenant 管理 API、Drive API、external integration API をまとめた OpenAPI document です。\n\n" +
			"## 認証\n\n" +
			"- browser API は `SESSION_ID` Cookie を使います。状態を変更する request では `X-CSRF-Token` も送ってください。\n" +
			"- external / M2M / SCIM API は Bearer token を使います。必要な audience、scope、role は環境設定で制御されます。\n" +
			"- tenant-scoped browser API は active tenant を前提にします。必要に応じて tenant select API で active tenant を切り替えます。\n\n" +
			"## Request の基本\n\n" +
			"- UUID は OpenAPI 上では `format: uuid` として表します。\n" +
			"- timestamp は RFC3339 UTC の `date-time` です。\n" +
			"- list API の `limit` / `offset` は pagination に使います。\n" +
			"- `Idempotency-Key` がある mutation API は、同じ key と同じ body の retry に対して保存済み response を返します。\n\n" +
			"## Error response\n\n" +
			"error は Problem Details 形式です。validation error、認証エラー、認可エラー、not found、conflict、rate limit、server error は `application/problem+json` の `default` response として表します。",
	)
}

func EnrichOpenAPIOperation(oapi *huma.OpenAPI, op *huma.Operation) {
	enrichOperationDescription(op)
	enrichParameters(op.Parameters)
	enrichRequestBody(oapi, op)
	enrichResponses(oapi, op)
	enrichComponentSchemas(oapi)
}

func enrichOperationDescription(op *huma.Operation) {
	if strings.TrimSpace(op.Description) != "" {
		return
	}
	if doc, ok := openAPIOperationDocs[op.OperationID]; ok && strings.TrimSpace(doc.Description) != "" {
		op.Description = doc.Description + operationUsageNotes(op)
		return
	}

	summary := strings.TrimSpace(op.Summary)
	if summary == "" {
		summary = fmt.Sprintf("%s %s", op.Method, op.Path)
	}
	op.Description = fmt.Sprintf("%s。\n\n%s%s", strings.TrimSuffix(summary, "。"), tagDescription(op), operationUsageNotes(op))
}

func tagDescription(op *huma.Operation) string {
	if len(op.Tags) == 0 {
		return "この endpoint は HaoHao API の操作を提供します。"
	}
	switch op.Tags[0] {
	case DocTagAuthSession:
		return "login、session refresh、CSRF token 取得、認証設定確認に使う endpoint です。"
	case DocTagTenantWorkspace:
		return "active tenant と tenant 内 workspace の日常操作に使う endpoint です。"
	case DocTagTenantAdministration:
		return "tenant admin 権限を持つ user が tenant lifecycle や support access を管理する endpoint です。"
	case DocTagCustomerSignals:
		return "customer signal の作成、検索、更新、import workflow に使う endpoint です。"
	case DocTagDataDatasets:
		return "file metadata、dataset、work table、query job、export の管理に使う endpoint です。"
	case DocTagPlatformIntegrations:
		return "integration、machine client、entitlement、webhook の管理に使う endpoint です。"
	case DocTagDriveFilesFolders:
		return "Drive の file/folder metadata、list、search、copy、trash などの基本操作に使う endpoint です。"
	case DocTagDriveSharingPermissions:
		return "Drive resource の共有、権限、share link、招待、group を管理する endpoint です。"
	case DocTagDriveCollaborationSync:
		return "Drive workspace、collaborative editing、office session、sync/offline workflow に使う endpoint です。"
	case DocTagDriveAIOCR:
		return "Drive file の OCR、product extraction、AI summary/classification に使う endpoint です。"
	case DocTagDriveSecurityCompliance:
		return "Drive の encryption、HSM、legal hold、eDiscovery、clean room、gateway を扱う compliance endpoint です。"
	case DocTagDriveAdminGovernance:
		return "tenant admin が Drive audit、policy、operations health、search index、content access を管理する endpoint です。"
	case DocTagExternalAPIs:
		return "browser Cookie に依存しない external client、M2M、SCIM integration 用 endpoint です。"
	default:
		return "この endpoint は HaoHao API の操作を提供します。"
	}
}

func operationUsageNotes(op *huma.Operation) string {
	notes := []string{}
	if hasSecurity(op, "cookieAuth") {
		notes = append(notes, "`SESSION_ID` Cookie が必要です。状態変更 request では `X-CSRF-Token` も送ります。")
	}
	if hasSecurity(op, "bearerAuth") || hasSecurity(op, "m2mBearerAuth") {
		notes = append(notes, "Bearer token が必要です。token の audience、scope、role は環境設定に従って検証されます。")
	}
	if op.RequestBody != nil {
		notes = append(notes, "request body は `application/json` を基本にし、必須 field と enum は schema に従って指定します。")
	}
	if len(notes) == 0 {
		return ""
	}
	return "\n\n### 使い方\n\n- " + strings.Join(notes, "\n- ")
}

func hasSecurity(op *huma.Operation, scheme string) bool {
	for _, requirement := range op.Security {
		if _, ok := requirement[scheme]; ok {
			return true
		}
	}
	return false
}

func enrichParameters(params []*huma.Param) {
	for _, param := range params {
		if param == nil {
			continue
		}
		if param.Description == "" {
			param.Description = parameterDescription(param)
		}
		if param.Example == nil && len(param.Examples) == 0 {
			param.Example = parameterExample(param)
		}
	}
}

func parameterDescription(param *huma.Param) string {
	switch param.Name {
	case "SESSION_ID":
		return "login または session refresh で発行される browser session Cookie です。"
	case "X-CSRF-Token":
		return "Cookie session を使う state-changing request に必要な CSRF token です。"
	case "Idempotency-Key":
		return "同じ request の retry を安全に扱うための任意 key です。同じ key は同じ method/path/body にだけ再利用できます。"
	case "tenantSlug", "X-Tenant-ID":
		return "tenant を識別する slug です。例: `acme`。"
	case "filePublicId":
		return "Drive file の public UUID です。"
	case "folderPublicId":
		return "Drive folder の public UUID です。"
	case "workspacePublicId":
		return "Drive workspace の public UUID です。"
	case "sharePublicId":
		return "Drive share record の public UUID です。"
	case "linkPublicId":
		return "Drive share link の public UUID です。"
	case "limit":
		return "返却件数の上限です。大量の結果は pagination で分割してください。"
	case "offset":
		return "pagination の開始位置です。`limit` と組み合わせて使います。"
	case "q", "query":
		return "検索語または filter query です。"
	default:
		switch param.In {
		case "path":
			return fmt.Sprintf("path 内の `%s` を指定します。", param.Name)
		case "query":
			return fmt.Sprintf("結果の絞り込みや pagination に使う query parameter `%s` です。", param.Name)
		case "header":
			return fmt.Sprintf("request header `%s` です。", param.Name)
		case "cookie":
			return fmt.Sprintf("request cookie `%s` です。", param.Name)
		default:
			return ""
		}
	}
}

func parameterExample(param *huma.Param) any {
	switch param.Name {
	case "SESSION_ID":
		return "sess_demo_1234567890"
	case "X-CSRF-Token":
		return "csrf_demo_1234567890"
	case "Idempotency-Key":
		return "req_20260115_customer_signal_001"
	case "tenantSlug", "X-Tenant-ID":
		return docsDemoTenantSlug
	case "filePublicId", "folderPublicId", "workspacePublicId", "sharePublicId", "linkPublicId", "invitationPublicId", "exportPublicId", "userPublicId":
		return docsDemoUUID
	case "limit":
		return 50
	case "offset":
		return 0
	case "q", "query":
		return "roadmap"
	default:
		if param.Schema != nil {
			return exampleForSchema(nil, param.Schema, param.Name, docsExampleRequest, 0)
		}
		return nil
	}
}

func enrichRequestBody(oapi *huma.OpenAPI, op *huma.Operation) {
	if op.RequestBody == nil {
		return
	}
	if op.RequestBody.Description == "" {
		op.RequestBody.Description = "request body には操作に必要な field を JSON で指定します。必須 field、enum、文字数制限は schema を参照してください。"
	}
	for contentType, media := range op.RequestBody.Content {
		if media == nil || !isJSONContentType(contentType) {
			continue
		}
		if len(media.Examples) > 0 || media.Example != nil {
			continue
		}
		example := requestExampleForOperation(oapi, op, media.Schema)
		if example == nil {
			continue
		}
		media.Examples = map[string]*huma.Example{
			"default": {
				Summary:     "Example request",
				Description: "ローカル demo tenant を想定した request body 例です。",
				Value:       example,
			},
		}
	}
}

func requestExampleForOperation(oapi *huma.OpenAPI, op *huma.Operation, schema *huma.Schema) any {
	if doc, ok := openAPIOperationDocs[op.OperationID]; ok && doc.RequestExample != nil {
		return doc.RequestExample
	}
	return exampleForSchema(oapi, schema, "", docsExampleRequest, 0)
}

func enrichResponses(oapi *huma.OpenAPI, op *huma.Operation) {
	for status, response := range op.Responses {
		if response == nil {
			continue
		}
		if status == "default" {
			if response.Description == "" || response.Description == "Error" {
				response.Description = "Problem Details 形式の error response です。validation、authentication、authorization、conflict、rate limit、server error などで返ります。"
			}
			enrichProblemResponse(response)
			continue
		}
		if response.Description == "" || response.Description == http.StatusText(statusCode(status)) {
			response.Description = successResponseDescription(status)
		}
		for contentType, media := range response.Content {
			if media == nil || !isJSONContentType(contentType) {
				continue
			}
			if len(media.Examples) > 0 || media.Example != nil {
				continue
			}
			example := responseExampleForOperation(oapi, op, media.Schema)
			if example == nil {
				continue
			}
			media.Examples = map[string]*huma.Example{
				"default": {
					Summary:     "Example response",
					Description: "成功時に返る response body の例です。",
					Value:       example,
				},
			}
		}
	}
}

func responseExampleForOperation(oapi *huma.OpenAPI, op *huma.Operation, schema *huma.Schema) any {
	if doc, ok := openAPIOperationDocs[op.OperationID]; ok && doc.ResponseExample != nil {
		return doc.ResponseExample
	}
	return exampleForSchema(oapi, schema, "", docsExampleResponse, 0)
}

func enrichProblemResponse(response *huma.Response) {
	if response.Content == nil {
		return
	}
	for contentType, media := range response.Content {
		if media == nil || !isJSONContentType(contentType) {
			continue
		}
		if len(media.Examples) > 0 || media.Example != nil {
			continue
		}
		media.Examples = map[string]*huma.Example{
			"validation": {
				Summary:     "Validation error",
				Description: "request body、path、query、header の validation に失敗した場合の例です。",
				Value: map[string]any{
					"type":   "https://example.com/problems/validation-error",
					"title":  "Unprocessable Entity",
					"status": 422,
					"detail": "Request validation failed.",
					"errors": []map[string]any{
						{
							"message":  "expected value to be at least 1 characters",
							"location": "body.name",
							"value":    "",
						},
					},
				},
			},
		}
	}
}

func successResponseDescription(status string) string {
	switch status {
	case "200":
		return "操作に成功し、response body に結果を返します。"
	case "201":
		return "resource の作成に成功し、作成された resource を返します。"
	case "202":
		return "request を受け付けました。処理結果は job/status API で確認します。"
	case "204":
		return "操作に成功しました。response body はありません。"
	default:
		return "操作に成功した場合の response です。"
	}
}

func statusCode(status string) int {
	var code int
	if _, err := fmt.Sscanf(status, "%d", &code); err != nil {
		return 0
	}
	return code
}

func isJSONContentType(contentType string) bool {
	return contentType == "application/json" || contentType == "application/problem+json" || strings.HasSuffix(contentType, "+json")
}

func enrichComponentSchemas(oapi *huma.OpenAPI) {
	if oapi == nil || oapi.Components == nil || oapi.Components.Schemas == nil {
		return
	}
	for _, schema := range oapi.Components.Schemas.Map() {
		enrichSchemaProperties(schema)
	}
}

func enrichSchemaProperties(schema *huma.Schema) {
	if schema == nil {
		return
	}
	for name, property := range schema.Properties {
		if property == nil {
			continue
		}
		if property.Description == "" {
			property.Description = schemaPropertyDescription(name)
		}
		if len(property.Examples) == 0 {
			if example := exampleForSchema(nil, property, name, docsExampleResponse, 0); example != nil {
				property.Examples = []any{example}
			}
		}
		enrichSchemaProperties(property)
	}
	if schema.Items != nil {
		enrichSchemaProperties(schema.Items)
	}
}

func schemaPropertyDescription(name string) string {
	switch name {
	case "publicId":
		return "client や URL path で参照する public UUID です。"
	case "id":
		return "内部 ID または protocol 上の識別子です。"
	case "tenantSlug":
		return "tenant を識別する slug です。"
	case "name", "displayName":
		return "画面表示や検索で使う名前です。"
	case "description":
		return "resource の説明文です。"
	case "createdAt", "updatedAt", "completedAt", "expiresAt", "deletedAt", "lockedAt", "occurredAt":
		return "RFC3339 UTC の timestamp です。"
	case "email", "inviteeEmail", "inviteeEmailNormalized", "userEmail":
		return "user または invitee の email address です。"
	case "role", "roleCode", "roles", "roleCodes":
		return "resource に対して付与する role または tenant role code です。"
	case "status":
		return "現在の lifecycle status です。"
	case "source":
		return "record の発生元または permission の由来です。"
	case "tags":
		return "検索や分類に使う tag 配列です。"
	case "limit":
		return "pagination で返す最大件数です。"
	case "offset":
		return "pagination の開始位置です。"
	case "filePublicId":
		return "対象 Drive file の public UUID です。"
	case "folderPublicId":
		return "対象 Drive folder の public UUID です。"
	case "workspacePublicId":
		return "対象 Drive workspace の public UUID です。"
	case "resourceType":
		return "`file` や `folder` など、対象 resource の種類です。"
	case "resourcePublicId":
		return "対象 resource の public UUID です。"
	case "canDownload":
		return "share link または permission で download を許可するかを表します。"
	case "password":
		return "share link 保護などに使う secret value です。response には返しません。"
	case "url":
		return "callback または external service の URL です。"
	default:
		return ""
	}
}

func exampleForSchema(oapi *huma.OpenAPI, schema *huma.Schema, name string, mode docsExampleMode, depth int) any {
	if schema == nil || depth > 6 {
		return nil
	}
	if schema.Ref != "" {
		if oapi == nil || oapi.Components == nil || oapi.Components.Schemas == nil {
			return exampleByName(name, schema.Format)
		}
		if resolved := schemaFromRef(oapi, schema.Ref); resolved != nil {
			return exampleForSchema(oapi, resolved, schemaNameFromRef(schema.Ref), mode, depth+1)
		}
		return exampleByName(name, schema.Format)
	}
	if len(schema.Examples) > 0 {
		return schema.Examples[0]
	}
	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}
	if len(schema.OneOf) > 0 {
		return exampleForSchema(oapi, schema.OneOf[0], name, mode, depth+1)
	}
	if len(schema.AnyOf) > 0 {
		return exampleForSchema(oapi, schema.AnyOf[0], name, mode, depth+1)
	}
	if len(schema.AllOf) > 0 {
		return exampleForSchema(oapi, schema.AllOf[0], name, mode, depth+1)
	}

	switch {
	case schema.Type == "object" || len(schema.Properties) > 0:
		return objectExample(oapi, schema, mode, depth)
	case schema.Type == "array":
		item := exampleForSchema(oapi, schema.Items, singularName(name), mode, depth+1)
		if item == nil {
			return []any{}
		}
		return []any{item}
	case schema.Type == "integer":
		return integerExample(name)
	case schema.Type == "number":
		return 42.5
	case schema.Type == "boolean":
		return booleanExample(name)
	case schema.Type == "string" || schema.Type == "":
		return stringExample(name, schema.Format)
	default:
		return exampleByName(name, schema.Format)
	}
}

func objectExample(oapi *huma.OpenAPI, schema *huma.Schema, mode docsExampleMode, depth int) map[string]any {
	example := map[string]any{}
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	required := map[string]struct{}{}
	for _, name := range schema.Required {
		required[name] = struct{}{}
	}
	for _, name := range names {
		property := schema.Properties[name]
		if property == nil {
			continue
		}
		if mode == docsExampleRequest && property.ReadOnly {
			continue
		}
		if mode == docsExampleResponse && property.WriteOnly {
			continue
		}
		if mode == docsExampleRequest {
			if _, ok := required[name]; !ok && isServerManagedField(name) {
				continue
			}
		}
		value := exampleForSchema(oapi, property, name, mode, depth+1)
		if value != nil {
			example[name] = value
		}
	}
	return example
}

func schemaFromRef(oapi *huma.OpenAPI, ref string) *huma.Schema {
	name := schemaNameFromRef(ref)
	if name == "" {
		return nil
	}
	return oapi.Components.Schemas.Map()[name]
}

func schemaNameFromRef(ref string) string {
	return path.Base(strings.TrimPrefix(ref, "#/components/schemas/"))
}

func singularName(name string) string {
	return strings.TrimSuffix(name, "s")
}

func isServerManagedField(name string) bool {
	switch name {
	case "id", "publicId", "createdAt", "updatedAt", "completedAt", "deletedAt", "lockedAt", "status", "createdByUserId", "token":
		return true
	default:
		return false
	}
}

func exampleByName(name, format string) any {
	switch format {
	case "uuid":
		return docsDemoUUID
	case "email":
		return docsDemoEmail
	case "uri":
		return "https://example.com/webhooks/haohao"
	case "date-time":
		return docsDemoTime
	case "date":
		return "2026-01-15"
	}
	return stringExample(name, format)
}

func stringExample(name, format string) string {
	switch format {
	case "uuid":
		return docsDemoUUID
	case "email":
		return docsDemoEmail
	case "uri":
		return "https://example.com/webhooks/haohao"
	case "date-time":
		return docsDemoTime
	case "date":
		return "2026-01-15"
	}

	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "publicid") || lower == "id" || strings.HasSuffix(lower, "id"):
		return docsDemoUUID
	case strings.Contains(lower, "tenant") && strings.Contains(lower, "slug"):
		return docsDemoTenantSlug
	case strings.Contains(lower, "email"):
		return docsDemoEmail
	case strings.Contains(lower, "url"):
		return "https://example.com/webhooks/haohao"
	case strings.Contains(lower, "role"):
		return "viewer"
	case strings.Contains(lower, "status"):
		return "active"
	case strings.Contains(lower, "source"):
		return "support"
	case strings.Contains(lower, "priority"):
		return "medium"
	case strings.Contains(lower, "title"):
		return "Export CSV from reports"
	case strings.Contains(lower, "body"):
		return "Customer asked for monthly report export."
	case strings.Contains(lower, "description"):
		return "Quarterly planning documents."
	case strings.Contains(lower, "filename"):
		return "roadmap.md"
	case strings.Contains(lower, "contenttype"):
		return "text/markdown"
	case strings.Contains(lower, "sha256"):
		return "3a6eb0790f39ac87c94f3856b2dd2c5d110e6811602261a9a923d3bb23adc8b7"
	case strings.Contains(lower, "password"):
		return "share-demo-passphrase"
	case strings.Contains(lower, "token"):
		return "token_demo_1234567890"
	case strings.Contains(lower, "query") || lower == "q":
		return "roadmap"
	case strings.Contains(lower, "name"):
		return "Acme"
	default:
		return "example"
	}
}

func integerExample(name string) int64 {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "count"):
		return 3
	case strings.Contains(lower, "limit"):
		return 50
	case strings.Contains(lower, "offset"):
		return 0
	case strings.Contains(lower, "bytes") || strings.Contains(lower, "size"):
		return 1048576
	default:
		return 1
	}
}

func booleanExample(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "blocked"), strings.Contains(lower, "deleted"), strings.Contains(lower, "revoked"):
		return false
	default:
		return true
	}
}
