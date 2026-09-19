package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestHistoryPermissionsAndReview verifies the server-side boundary: an
// administrator can review only assigned domains, while a super administrator
// can review all domains. The transaction is always rolled back.
func TestHistoryPermissionsAndReview(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	var undergraduate, academicGraduate, mpa model.DataDomain
	if err := db.Where("code = ?", common.DataDomainUndergraduate).First(&undergraduate).Error; err != nil {
		t.Fatalf("find undergraduate domain: %v", err)
	}
	if err := db.Where("code = ?", common.DataDomainAcademicGraduate).First(&academicGraduate).Error; err != nil {
		t.Fatalf("find academic graduate domain: %v", err)
	}
	if err := db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA domain: %v", err)
	}

	rollback := errors.New("rollback history permission test transaction")
	err = db.Transaction(func(tx *gorm.DB) error {
		ctx := context.Background()
		tag := fmt.Sprintf("history-permission-%d", time.Now().UnixNano())
		undergraduateID, academicGraduateID, mpaID := undergraduate.ID, academicGraduate.ID, mpa.ID
		undergraduateContribution := &model.HistoryContribution{
			Title: tag + "-undergraduate", Content: "本科投稿内容", SourceNote: "测试来源",
			Status: repository.HistoryContributionPending, DataDomainID: &undergraduateID, AuthorUserID: 1101, AuthorAlumniID: 2101,
		}
		mpaContribution := &model.HistoryContribution{
			Title: tag + "-mpa", Content: "MPA 投稿内容", SourceNote: "测试来源",
			Status: repository.HistoryContributionPending, DataDomainID: &mpaID, AuthorUserID: 1102, AuthorAlumniID: 2102,
		}
		draft := &model.HistoryContribution{
			Title: tag + "-draft", Content: "草稿内容", SourceNote: "测试来源",
			Status: repository.HistoryContributionDraft, DataDomainID: &mpaID, AuthorUserID: 1102, AuthorAlumniID: 2102,
		}
		returned := &model.HistoryContribution{
			Title: tag + "-returned", Content: "待补充内容", SourceNote: "测试来源",
			Status: repository.HistoryContributionPending, DataDomainID: &mpaID, AuthorUserID: 1102, AuthorAlumniID: 2102,
		}
		rejected := &model.HistoryContribution{
			Title: tag + "-rejected", Content: "待驳回内容", SourceNote: "测试来源",
			Status: repository.HistoryContributionPending, DataDomainID: &mpaID, AuthorUserID: 1102, AuthorAlumniID: 2102,
		}
		unassignedContribution := &model.HistoryContribution{
			Title: tag + "-unassigned", Content: "未分配培养类别的投稿内容", SourceNote: "测试来源",
			Status: repository.HistoryContributionPending, AuthorUserID: 1104, AuthorAlumniID: 2104,
		}
		for _, contribution := range []*model.HistoryContribution{undergraduateContribution, mpaContribution, draft, returned, rejected, unassignedContribution} {
			if err := tx.Create(contribution).Error; err != nil {
				return err
			}
		}
		pendingAttachment := &model.HistoryAttachment{ContributionID: mpaContribution.ID, ObjectKey: tag + "/attachment.pdf", OriginalName: "review.pdf", MimeType: "application/pdf", Description: "测试附件", SourceNote: "测试来源", RightsNote: "测试授权", ConsentConfirmed: true, Status: "pending"}
		rejectedAttachment := &model.HistoryAttachment{ContributionID: rejected.ID, ObjectKey: tag + "/rejected.pdf", OriginalName: "rejected.pdf", MimeType: "application/pdf", Description: "测试附件", SourceNote: "测试来源", RightsNote: "测试授权", ConsentConfirmed: true, Status: "pending"}
		if err := tx.Create(pendingAttachment).Error; err != nil {
			return err
		}
		if err := tx.Create(rejectedAttachment).Error; err != nil {
			return err
		}

		svc := NewHistoryService(repository.NewHistoryRepository(tx), nil)
		mpaAdmin := common.AccessContext{UserID: 7001, Role: common.RoleAdmin, DomainIDs: []uint64{mpaID}}
		multiDomainAdmin := common.AccessContext{UserID: 7003, Role: common.RoleAdmin, DomainIDs: []uint64{undergraduateID, mpaID}}
		unassignedAdmin := common.AccessContext{UserID: 7004, Role: common.RoleAdmin}
		superAdmin := common.AccessContext{UserID: 7002, Role: common.RoleSuperAdmin}
		alumni := common.AccessContext{UserID: 1102, Role: common.RoleAlumni}
		otherAlumni := common.AccessContext{UserID: 1103, Role: common.RoleAlumni}
		// Exercise the same domain lookup and insert path used by a logged-in
		// alumnus submitting a new contribution. First use the seeded E2E alumnus
		// (the exact account used by the browser flow), then a fresh profile. The
		// permission fixtures above insert contributions directly, which would
		// otherwise miss this path.
		var seededProfile model.AlumniProfile
		seededMobile := "13800001111"
		if err := tx.Where("mobile = ?", seededMobile).First(&seededProfile).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			seededProfile = model.AlumniProfile{
				DataDomainID: mpaID, Name: tag + "-seeded-author", Grade: "2020", Mobile: &seededMobile, Status: "active",
			}
			if err := tx.Create(&seededProfile).Error; err != nil {
				return err
			}
		}
		var seededUser model.User
		if err := tx.Where("alumni_id = ?", seededProfile.ID).First(&seededUser).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			seededUser = model.User{
				Account: tag + "-seeded-user", PasswordHash: "not-used-in-this-test", Role: common.RoleAlumni,
				AlumniID: &seededProfile.ID, Status: "active",
			}
			if err := tx.Create(&seededUser).Error; err != nil {
				return err
			}
		}
		seeded, err := svc.CreateDraft(ctx, common.AccessContext{
			UserID: seededUser.ID, Role: common.RoleAlumni, AlumniID: &seededProfile.ID,
		}, dto.HistoryContributionRequest{
			Title: tag + "-seeded", Content: "种子校友投稿创建路径", SourceNote: "测试来源",
		})
		if err != nil || seeded == nil || seeded.Status != repository.HistoryContributionDraft || seeded.DataDomainID == nil || *seeded.DataDomainID != mpaID {
			t.Errorf("seeded alumni create draft = %+v, err %v; want MPA draft", seeded, err)
		}

		profile := &model.AlumniProfile{
			DataDomainID: mpaID,
			Name:         tag + "-author",
			Grade:        "2020",
			Status:       "active",
		}
		if err := tx.Create(profile).Error; err != nil {
			return err
		}
		created, err := svc.CreateDraft(ctx, common.AccessContext{
			UserID: 1201, Role: common.RoleAlumni, AlumniID: &profile.ID,
		}, dto.HistoryContributionRequest{
			Title: tag + "-created", Content: "真实投稿创建路径", SourceNote: "测试来源",
		})
		if err != nil || created == nil || created.Status != repository.HistoryContributionDraft || created.DataDomainID == nil || *created.DataDomainID != mpaID {
			t.Errorf("alumni create draft = %+v, err %v; want MPA draft", created, err)
		}
		undergraduateProfile := &model.AlumniProfile{DataDomainID: undergraduateID, Name: tag + "-undergraduate-author", Grade: "2020", Status: "active"}
		if err := tx.Create(undergraduateProfile).Error; err != nil {
			return err
		}
		undergraduateDraft, err := svc.CreateDraft(ctx, common.AccessContext{
			UserID: 1202, Role: common.RoleAlumni, AlumniID: &undergraduateProfile.ID,
		}, dto.HistoryContributionRequest{Title: tag + "-undergraduate-draft", Content: "本科校友投稿", SourceNote: "测试来源"})
		if err != nil || undergraduateDraft == nil || undergraduateDraft.DataDomainID == nil || *undergraduateDraft.DataDomainID != undergraduateID {
			t.Errorf("undergraduate alumni draft = %+v, err %v; want assigned undergraduate domain", undergraduateDraft, err)
		}
		academicGraduateProfile := &model.AlumniProfile{DataDomainID: academicGraduateID, Name: tag + "-academic-graduate-author", Grade: "2020", Status: "active"}
		if err := tx.Create(academicGraduateProfile).Error; err != nil {
			return err
		}
		academicGraduateDraft, err := svc.CreateDraft(ctx, common.AccessContext{
			UserID: 1203, Role: common.RoleAlumni, AlumniID: &academicGraduateProfile.ID,
		}, dto.HistoryContributionRequest{Title: tag + "-academic-graduate-draft", Content: "学术学位研究生校友投稿", SourceNote: "测试来源"})
		if err != nil || academicGraduateDraft == nil || academicGraduateDraft.DataDomainID == nil || *academicGraduateDraft.DataDomainID != academicGraduateID {
			t.Errorf("academic graduate alumni draft = %+v, err %v; want assigned academic graduate domain", academicGraduateDraft, err)
		}

		pending, err := svc.ListPending(ctx, mpaAdmin)
		assertHistoryContributionIDs(t, "MPA pending", pending, mpaContribution.ID, returned.ID, rejected.ID)
		if err != nil {
			t.Errorf("MPA pending error = %v", err)
		}
		allPending, err := svc.ListPending(ctx, superAdmin)
		assertHistoryContributionIDs(t, "super-admin pending", allPending, undergraduateContribution.ID, mpaContribution.ID, returned.ID, rejected.ID, unassignedContribution.ID)
		if err != nil {
			t.Errorf("super-admin pending error = %v", err)
		}
		multiPending, err := svc.ListPending(ctx, multiDomainAdmin)
		assertHistoryContributionIDs(t, "multi-domain pending", multiPending, undergraduateContribution.ID, mpaContribution.ID, returned.ID, rejected.ID)
		if err != nil {
			t.Errorf("multi-domain pending error = %v", err)
		}
		unassignedPending, err := svc.ListPending(ctx, unassignedAdmin)
		assertHistoryContributionIDs(t, "unassigned administrator pending", unassignedPending)
		if err != nil {
			t.Errorf("unassigned administrator pending error = %v", err)
		}
		mine, err := svc.ListMine(ctx, alumni)
		if err != nil || len(mine) != 4 {
			t.Errorf("alumni mine count = %d, err %v; want 4 own contributions", len(mine), err)
		}
		if _, err := svc.ListMine(ctx, mpaAdmin); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("admin ListMine error = %v, want permission denied", err)
		}
		attachments, err := svc.ListAttachments(ctx, mpaAdmin, mpaContribution.ID)
		if err != nil || len(attachments) != 1 || attachments[0].OriginalName != "review.pdf" {
			t.Errorf("in-domain attachments = %+v, err %v", attachments, err)
		}
		if _, err := svc.ListAttachments(ctx, mpaAdmin, undergraduateContribution.ID); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("out-of-domain attachments error = %v, want permission denied", err)
		}
		if _, err := svc.ListAttachments(ctx, alumni, mpaContribution.ID); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("alumni attachments error = %v, want permission denied", err)
		}
		if _, err := svc.AttachmentDownloadURL(ctx, otherAlumni, mpaContribution.ID, pendingAttachment.ID); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("other alumni pending attachment = %v, want permission denied", err)
		}
		if _, err := svc.AttachmentDownloadURL(ctx, alumni, mpaContribution.ID, pendingAttachment.ID); !errors.Is(err, common.ErrStorageUnavailable) {
			t.Errorf("author pending attachment = %v, want storage unavailable after authorization", err)
		}
		if _, err := svc.Review(ctx, unassignedAdmin, mpaContribution.ID, dto.HistoryReviewRequest{Action: "approve"}); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("unassigned admin review error = %v, want permission denied", err)
		}
		if _, err := svc.Review(ctx, mpaAdmin, unassignedContribution.ID, dto.HistoryReviewRequest{Action: "approve"}); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("domain administrator review of unassigned contribution = %v, want permission denied", err)
		}
		if _, err := svc.Review(ctx, superAdmin, unassignedContribution.ID, dto.HistoryReviewRequest{Action: "reject", ReviewComment: "请管理员补充分配信息"}); err != nil {
			t.Errorf("super-admin review of unassigned contribution: %v", err)
		}
		if _, err := svc.Review(ctx, multiDomainAdmin, undergraduateContribution.ID, dto.HistoryReviewRequest{Action: "approve"}); err != nil {
			t.Errorf("multi-domain admin cross-domain approve: %v", err)
		}
		returnedResult, err := svc.Review(ctx, mpaAdmin, returned.ID, dto.HistoryReviewRequest{Action: "return", ReviewComment: "请补充来源"})
		if err != nil || returnedResult.Status != repository.HistoryContributionReturned {
			t.Errorf("return result = %+v, err %v; want returned", returnedResult, err)
		}
		if _, err := svc.Submit(ctx, otherAlumni, returned.ID); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("other alumnus submit returned contribution = %v, want permission denied", err)
		}
		resubmitted, err := svc.Submit(ctx, alumni, returned.ID)
		if err != nil || resubmitted.Status != repository.HistoryContributionPending {
			t.Errorf("returned resubmit = %+v, err %v; want pending", resubmitted, err)
		}
		rejectedResult, err := svc.Review(ctx, mpaAdmin, rejected.ID, dto.HistoryReviewRequest{Action: "reject", ReviewComment: "资料不完整"})
		if err != nil || rejectedResult.Status != repository.HistoryContributionRejected {
			t.Errorf("reject result = %+v, err %v; want rejected", rejectedResult, err)
		}
		if _, err := svc.Review(ctx, mpaAdmin, rejected.ID, dto.HistoryReviewRequest{Action: "approve"}); !errors.Is(err, common.ErrInvalidHistoryState) {
			t.Errorf("re-review rejected contribution = %v, want invalid state", err)
		}
		if _, err := svc.Review(ctx, mpaAdmin, undergraduateContribution.ID, dto.HistoryReviewRequest{Action: "approve"}); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("out-of-domain review error = %v, want permission denied", err)
		}
		if _, err := svc.Review(ctx, mpaAdmin, mpaContribution.ID, dto.HistoryReviewRequest{Action: "return"}); !errors.Is(err, common.ErrInvalidRequest) {
			t.Errorf("return without comment error = %v, want invalid request", err)
		}
		approved, err := svc.Review(ctx, mpaAdmin, mpaContribution.ID, dto.HistoryReviewRequest{Action: "approve"})
		if err != nil || approved.Status != repository.HistoryContributionApproved || approved.EntryID == nil {
			t.Errorf("approve result = %+v, err %v; want approved contribution with entry", approved, err)
		}
		if _, err := svc.Review(ctx, superAdmin, returned.ID, dto.HistoryReviewRequest{Action: "reject", ReviewComment: "资料不完整"}); err != nil {
			t.Errorf("super-admin review after resubmission: %v", err)
		}
		var versions int64
		if err := tx.Model(&model.HistoryEntryVersion{}).Where("contribution_id = ?", mpaContribution.ID).Count(&versions).Error; err != nil {
			return err
		}
		if versions != 1 {
			t.Errorf("approved contribution versions = %d, want 1", versions)
		}
		var rejectedVersions int64
		if err := tx.Model(&model.HistoryEntryVersion{}).Where("contribution_id = ?", rejected.ID).Count(&rejectedVersions).Error; err != nil {
			return err
		}
		if rejectedVersions != 0 {
			t.Errorf("rejected contribution versions = %d, want 0", rejectedVersions)
		}
		var approvedAttachment, rejectedAttachmentState model.HistoryAttachment
		if err := tx.First(&approvedAttachment, pendingAttachment.ID).Error; err != nil {
			return err
		}
		if approvedAttachment.Status != "approved" {
			t.Errorf("approved attachment status = %q, want approved", approvedAttachment.Status)
		}
		if err := tx.First(&rejectedAttachmentState, rejectedAttachment.ID).Error; err != nil {
			return err
		}
		if rejectedAttachmentState.Status != "rejected" {
			t.Errorf("rejected attachment status = %q, want rejected", rejectedAttachmentState.Status)
		}
		if _, err := svc.AttachmentDownloadURL(ctx, otherAlumni, mpaContribution.ID, pendingAttachment.ID); !errors.Is(err, common.ErrStorageUnavailable) {
			t.Errorf("other alumni approved attachment = %v, want storage unavailable after public authorization", err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("history permission transaction: %v", err)
	}
}

func assertHistoryContributionIDs(t *testing.T, label string, items []dto.HistoryContributionItem, want ...uint64) {
	t.Helper()
	if len(items) != len(want) {
		t.Errorf("%s count = %d, want %d", label, len(items), len(want))
		return
	}
	seen := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		seen[item.ID] = struct{}{}
	}
	for _, id := range want {
		if _, ok := seen[id]; !ok {
			t.Errorf("%s does not contain contribution %d", label, id)
		}
	}
}
