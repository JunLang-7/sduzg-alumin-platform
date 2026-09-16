package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAlumniFileUploadDownloadDeleteE2E(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	minioEndpoint := os.Getenv("TEST_MINIO_ENDPOINT")
	if dsn == "" || redisAddr == "" || minioEndpoint == "" {
		t.Skip("TEST_MYSQL_DSN, TEST_REDIS_ADDR, and TEST_MINIO_ENDPOINT are required")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr, DB: 14})
	t.Cleanup(func() { _ = redisClient.Close() })
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping test redis: %v", err)
	}

	storageClient, err := storage.New(config.StorageConfig{
		Enabled:   true,
		Endpoint:  minioEndpoint,
		AccessKey: os.Getenv("TEST_MINIO_ACCESS_KEY"),
		SecretKey: os.Getenv("TEST_MINIO_SECRET_KEY"),
		Bucket:    os.Getenv("TEST_MINIO_BUCKET"),
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("create MinIO client: %v", err)
	}

	var operatorID, alumniID uint64
	var objectKey string
	t.Cleanup(func() {
		if objectKey != "" {
			_ = storageClient.DeleteFile(context.Background(), objectKey)
		}
		if operatorID != 0 {
			_ = db.Where("operator_id = ?", operatorID).Delete(&model.OperationLog{}).Error
		}
		if alumniID != 0 {
			_ = db.Unscoped().Where("alumni_id = ?", alumniID).Delete(&model.AlumniFile{}).Error
			_ = db.Unscoped().Where("id = ?", alumniID).Delete(&model.AlumniProfile{}).Error
		}
		if operatorID != 0 {
			_ = db.Unscoped().Where("id = ?", operatorID).Delete(&model.User{}).Error
		}
	})

	var domain model.DataDomain
	if err := db.Where("code = ? AND status = ?", common.DataDomainMPA, common.DataDomainStatusActive).First(&domain).Error; err != nil {
		t.Fatalf("find MPA data domain: %v", err)
	}

	suffix := time.Now().UnixNano()
	operator := &model.User{
		Account:      fmt.Sprintf("file-e2e-%d", suffix),
		PasswordHash: "not-used-by-e2e",
		Role:         common.RoleSuperAdmin,
		RealName:     ptrString("文件集成测试管理员"),
		Status:       common.UserStatusActive,
	}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create e2e operator: %v", err)
	}
	operatorID = operator.ID
	profile := &model.AlumniProfile{
		DataDomainID: domain.ID,
		Name:         fmt.Sprintf("文件集成测试校友-%d", suffix),
		Grade:        "2020级",
		Status:       common.AlumniStatusActive,
		CreatedBy:    &operatorID,
		UpdatedBy:    &operatorID,
	}
	if err := db.Create(profile).Error; err != nil {
		t.Fatalf("create e2e alumni: %v", err)
	}
	alumniID = profile.ID

	gin.SetMode(gin.TestMode)
	secret := "alumni-file-e2e-secret"
	engine := New(Dependencies{
		Config: config.Config{
			App:  config.AppConfig{Name: "alumni-file-e2e", Env: config.EnvDevelopment},
			Auth: config.AuthConfig{JWTSecret: secret, AccessTokenTTL: time.Hour},
		},
		Logger:        zap.NewNop(),
		DB:            db,
		RedisClient:   redisClient,
		StorageClient: storageClient,
	})
	token := testAccessTokenForUser(t, secret, operatorID, time.Now().Add(time.Hour))

	assertAPIStatus(t, engine, http.MethodGet, "/api/v1/health/ready", "", "", http.StatusOK)

	uploadResponse := performAPIRequest(t, engine, http.MethodPost,
		fmt.Sprintf("/api/v1/admin/alumni/%d/files/upload-url", alumniID),
		`{"file_type":"degree_archive","original_name":"degree.pdf","mime_type":"application/pdf"}`,
		token,
	)
	if uploadResponse.Code != http.StatusOK {
		t.Fatalf("request upload URL status = %d, body = %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	var uploadBody struct {
		Code int `json:"code"`
		Data struct {
			FileID    uint64 `json:"file_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
	}
	decodeAPIResponse(t, uploadResponse, &uploadBody)
	if uploadBody.Code != 0 || uploadBody.Data.FileID == 0 || uploadBody.Data.UploadURL == "" {
		t.Fatalf("unexpected upload URL response: %s", uploadResponse.Body.String())
	}

	content := []byte("MinIO integration upload payload")
	putRequest, err := http.NewRequestWithContext(context.Background(), http.MethodPut, uploadBody.Data.UploadURL, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("build presigned upload request: %v", err)
	}
	putRequest.Header.Set("Content-Type", "application/pdf")
	putResponse, err := http.DefaultClient.Do(putRequest)
	if err != nil {
		t.Fatalf("upload file to MinIO: %v", err)
	}
	if putResponse.StatusCode != http.StatusOK {
		_ = putResponse.Body.Close()
		t.Fatalf("MinIO upload status = %d", putResponse.StatusCode)
	}
	_ = putResponse.Body.Close()

	confirmResponse := performAPIRequest(t, engine, http.MethodPost,
		fmt.Sprintf("/api/v1/admin/alumni/%d/files/%d/confirm", alumniID, uploadBody.Data.FileID), "", token)
	if confirmResponse.Code != http.StatusOK {
		t.Fatalf("confirm upload status = %d, body = %s", confirmResponse.Code, confirmResponse.Body.String())
	}
	var confirmBody struct {
		Code int `json:"code"`
		Data struct {
			FileSize uint64 `json:"file_size"`
		} `json:"data"`
	}
	decodeAPIResponse(t, confirmResponse, &confirmBody)
	if confirmBody.Code != 0 || confirmBody.Data.FileSize != uint64(len(content)) {
		t.Fatalf("unexpected confirm response: %s", confirmResponse.Body.String())
	}

	var file model.AlumniFile
	if err := db.Where("id = ?", uploadBody.Data.FileID).First(&file).Error; err != nil {
		t.Fatalf("find confirmed file record: %v", err)
	}
	objectKey = file.ObjectKey
	if file.Status != common.FileStatusActive {
		t.Fatalf("file status = %q, want %q", file.Status, common.FileStatusActive)
	}

	downloadResponse := performAPIRequest(t, engine, http.MethodGet,
		fmt.Sprintf("/api/v1/admin/alumni/%d/files/%d/download", alumniID, file.ID), "", token)
	if downloadResponse.Code != http.StatusOK {
		t.Fatalf("request download URL status = %d, body = %s", downloadResponse.Code, downloadResponse.Body.String())
	}
	var downloadBody struct {
		Code int `json:"code"`
		Data struct {
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}
	decodeAPIResponse(t, downloadResponse, &downloadBody)
	getResponse, err := http.Get(downloadBody.Data.DownloadURL)
	if err != nil {
		t.Fatalf("download file from MinIO: %v", err)
	}
	downloaded, readErr := io.ReadAll(getResponse.Body)
	_ = getResponse.Body.Close()
	if readErr != nil || getResponse.StatusCode != http.StatusOK || !bytes.Equal(downloaded, content) {
		t.Fatalf("MinIO download = status %d, body %q, read error %v", getResponse.StatusCode, downloaded, readErr)
	}

	deleteResponse := performAPIRequest(t, engine, http.MethodDelete,
		fmt.Sprintf("/api/v1/admin/alumni/%d/files/%d", alumniID, file.ID), "", token)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete file status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, err := storageClient.StatObject(context.Background(), objectKey); err == nil {
		t.Fatal("deleted file still exists in MinIO")
	}
	objectKey = ""
}

func performAPIRequest(t *testing.T, engine *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func decodeAPIResponse(t *testing.T, rec *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), destination); err != nil {
		t.Fatalf("decode API response %q: %v", rec.Body.String(), err)
	}
}

func assertAPIStatus(t *testing.T, engine *gin.Engine, method, path, body, token string, want int) {
	t.Helper()
	response := performAPIRequest(t, engine, method, path, body, token)
	if response.Code != want {
		t.Fatalf("%s %s status = %d, want %d; body = %s", method, path, response.Code, want, response.Body.String())
	}
}
