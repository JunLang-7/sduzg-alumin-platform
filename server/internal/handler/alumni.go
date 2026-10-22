package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type AlumniHandler struct {
	alumni *service.AlumniService
}

func NewAlumniHandler(alumni *service.AlumniService) *AlumniHandler {
	return &AlumniHandler{alumni: alumni}
}

// List 按数据域分页查询校友档案。
//
//	@Summary		校友档案列表
//	@Description	管理员按已分配的有效数据域分页查询校友档案。无 alumni.sensitive.read 功能权限时，mobile、work_unit、position 等敏感字段返回 null，也不能用敏感字段筛选。
//	@Tags			校友档案查询与管理
//	@Produce		json
//	@Param			page			query		int		false	"页码，默认 1"
//	@Param			page_size		query		int		false	"每页条数，默认 20，最大 100"
//	@Param			keyword			query		string	false	"模糊匹配姓名等公开字段"
//	@Param			grade			query		string	false	"年级"
//	@Param			class_name		query		string	false	"班级"
//	@Param			cohort			query		string	false	"届次"
//	@Param			counselor		query		string	false	"辅导员"
//	@Param			mentor			query		string	false	"导师"
//	@Param			major			query		string	false	"专业"
//	@Param			training_mode	query		string	false	"培养方式"
//	@Param			industry		query		string	false	"行业"
//	@Param			work_unit		query		string	false	"工作单位；需 alumni.sensitive.read 权限"
//	@Param			position		query		string	false	"职务；需 alumni.sensitive.read 权限"
//	@Param			mobile			query		string	false	"手机号；需 alumni.sensitive.read 权限"
//	@Param			data_domain_id	query		integer	false	"数据域 ID；不传则在调用者可用范围内查询"
//	@Success		200				{object}	response.Body{data=dto.AlumniListItemPage}
//	@Failure		401				{object}	response.Body	"未登录或令牌失效"
//	@Failure		403				{object}	response.Body	"非管理员，或数据域不在可访问范围"
//	@Failure		503				{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/alumni [get]
func (h *AlumniHandler) List(c *gin.Context) {
	var req dto.AlumniListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.alumni.List(c.Request.Context(), req, *access)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Detail 按 ID 查询校友档案详情。
//
//	@Summary		校友档案详情
//	@Description	管理员调用，校友不能通过本接口查询他人或本人档案（本人资料使用 GET /alumni/me）。无 alumni.sensitive.read 权限时敏感字段返回 null。
//	@Tags			校友档案查询与管理
//	@Produce		json
//	@Param			id	path		integer	true	"校友档案 ID"
//	@Success		200	{object}	response.Body{data=dto.AlumniDetail}
//	@Failure		400	{object}	response.Body	"校友 ID 无效"
//	@Failure		403	{object}	response.Body	"权限不足，或档案不在可访问数据域内"
//	@Failure		404	{object}	response.Body	"对应校友不存在"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/alumni/{id} [get]
func (h *AlumniHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.alumni.GetByID(c.Request.Context(), id, *access)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Create 新增校友档案，成功返回 HTTP 201。
//
//	@Summary		新增校友档案
//	@Description	管理员在已授权数据域内新增档案。写入敏感字段需要 alumni.sensitive.read 功能权限。
//	@Tags			校友档案查询与管理
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.AdminAlumniCreateRequest	true	"档案字段；name 与 grade 必填"
//	@Success		201		{object}	response.Body{data=dto.AlumniDetail}
//	@Failure		400		{object}	response.Body	"参数不正确或数据域无效"
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		503		{object}	response.Body	"数据库或数据域不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni [post]
func (h *AlumniHandler) Create(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.AdminAlumniCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.Create(c.Request.Context(), *access, req)
	if err == nil {
		response.JSON(c, http.StatusCreated, response.CodeSuccess, "success", result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrInvalidDataDomain):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "数据域无效")
	case errors.Is(err, common.ErrDataDomainUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "数据域不可用")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Update 按 ID 更新校友档案。
//
//	@Summary		更新校友档案
//	@Description	管理员在已授权数据域内更新档案。指针字段为空时不修改，传入空字符串则清空。
//	@Tags			校友档案查询与管理
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer							true	"校友档案 ID"
//	@Param			request	body		dto.AdminAlumniUpdateRequest	true	"档案字段；name 与 grade 必填"
//	@Success		200		{object}	response.Body{data=dto.AlumniDetail}
//	@Failure		400		{object}	response.Body	"参数不正确"
//	@Failure		403		{object}	response.Body	"权限不足，或档案不在可访问数据域内"
//	@Failure		404		{object}	response.Body	"对应校友不存在"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/{id} [put]
func (h *AlumniHandler) Update(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	var req dto.AdminAlumniUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.Update(c.Request.Context(), *access, id, req)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Delete 按 ID 删除校友档案，并级联删除其档案文件。
//
//	@Summary	删除校友档案
//	@Tags		校友档案查询与管理
//	@Produce	json
//	@Param		id	path		integer	true	"校友档案 ID"
//	@Success	200	{object}	response.Body{data=dto.DeletedResult}
//	@Failure	403	{object}	response.Body	"权限不足，或档案不在可访问数据域内"
//	@Failure	404	{object}	response.Body	"对应校友不存在"
//	@Failure	503	{object}	response.Body	"数据库不可用"
//	@Security	BearerAuth
//	@Router		/admin/alumni/{id} [delete]
func (h *AlumniHandler) Delete(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	err = h.alumni.Delete(c.Request.Context(), *access, id)
	if err == nil {
		response.Success(c, dto.DeletedResult{Deleted: true})
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Export 按当前筛选条件导出校友档案，成功返回文件流而不使用 JSON envelope。
//
//	@Summary		导出校友档案
//	@Description	导出当前筛选结果。无 alumni.sensitive.read 权限时，导出内容不包含受限字段，导出列留空。响应为文件流，失败仍返回 JSON。
//	@Tags			校友导入导出
//	@Produce		application/octet-stream
//	@Param			format			query		string			false	"导出格式，csv 或 xlsx，默认 xlsx"
//	@Param			keyword			query		string			false	"模糊匹配姓名等公开字段"
//	@Param			grade			query		string			false	"年级"
//	@Param			class_name		query		string			false	"班级"
//	@Param			cohort			query		string			false	"届次"
//	@Param			counselor		query		string			false	"辅导员"
//	@Param			mentor			query		string			false	"导师"
//	@Param			major			query		string			false	"专业"
//	@Param			training_mode	query		string			false	"培养方式"
//	@Param			industry		query		string			false	"行业"
//	@Param			work_unit		query		string			false	"工作单位；需 alumni.sensitive.read 权限"
//	@Param			position		query		string			false	"职务；需 alumni.sensitive.read 权限"
//	@Param			mobile			query		string			false	"手机号；需 alumni.sensitive.read 权限"
//	@Param			data_domain_id	query		integer			false	"数据域 ID"
//	@Success		200				{file}		binary			"Excel 或 CSV 文件流"
//	@Failure		403				{object}	response.Body	"权限不足"
//	@Failure		503				{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/export [get]
func (h *AlumniHandler) Export(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.AlumniExportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.Export(c.Request.Context(), req, *access)
	if err == nil {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
		c.Data(http.StatusOK, result.ContentType, result.Data)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// ExportTemplate 下载导入模板，成功返回文件流而不使用 JSON envelope。
//
//	@Summary	下载导入模板
//	@Tags		校友导入导出
//	@Produce	application/octet-stream
//	@Success	200	{file}		binary			"导入模板文件流"
//	@Failure	500	{object}	response.Body	"导出模板生成失败"
//	@Security	BearerAuth
//	@Router		/admin/alumni/template [get]
func (h *AlumniHandler) ExportTemplate(c *gin.Context) {
	result, err := h.alumni.ExportTemplate(c.Request.Context())
	if err == nil {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
		c.Data(http.StatusOK, result.ContentType, result.Data)
		return
	}

	c.Error(err)
	response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "导出模板生成失败")
}

// Import 解析 Excel 文件批量导入校友档案。
//
//	@Summary		批量导入校友档案
//	@Description	使用导出的模板文件（.xlsx）或导出的 CSV 文件导入。单次最多 20000 条，每行独立校验，导入结果逐行返回错误原因；写入敏感字段需要 alumni.sensitive.read 功能权限。
//	@Tags			校友导入导出
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			file			formData	file	true	"Excel 模板（.xlsx）或 CSV（.csv）文件，最大 20MB"
//	@Param			data_domain_id	formData	integer	false	"目标数据域 ID；不传则使用调用者可用的数据域"
//	@Success		200				{object}	response.Body{data=dto.AlumniImportResult}
//	@Failure		400				{object}	response.Body	"未选择文件、文件过大、格式不正确、数据域无效，或超过 20000 条限制"
//	@Failure		403				{object}	response.Body	"权限不足"
//	@Failure		503				{object}	response.Body	"数据库或数据域不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/import [post]
func (h *AlumniHandler) Import(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	var dataDomainID *uint64
	if value, exists := c.GetPostForm("data_domain_id"); exists {
		id, err := strconv.ParseUint(value, 10, 64)
		if err != nil || id == 0 {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "数据域 ID 无效")
			return
		}
		dataDomainID = &id
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请选择要上传的 Excel 文件")
		return
	}

	const maxSize = 20 << 20 // 20 MB
	if fileHeader.Size > maxSize {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "文件过大，请上传小于 20MB 的文件")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "无法打开上传文件")
		return
	}
	defer file.Close()

	result, err := h.alumni.Import(c.Request.Context(), *access, dataDomainID, file)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrDataDomainUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "数据域不可用")
	case errors.Is(err, common.ErrInvalidDataDomain):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "数据域无效")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	case errors.Is(err, common.ErrImportInvalidHeader):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "表头不正确，应使用 15 列通用模板或 16 列含所属领域模板")
	case errors.Is(err, common.ErrImportNoDataRows):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "文件里没有可导入的数据行（只有表头或空白行）")
	case errors.Is(err, common.ErrImportDomainRequired):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "未指定培养类别：请在导入时选择目标培养类别，或使用含“所属领域”列的模板")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "文件格式不正确，请使用导出的模板文件")
	case errors.Is(err, common.ErrImportRowLimit):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "单次最多导入 20000 条数据，请拆分文件后重试")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "服务器内部错误")
	}
}

// Me 返回当前登录校友的本人资料。
//
//	@Summary		获取本人资料
//	@Description	仅校友本人可调用，返回本人完整档案；本人资料读取不屏蔽敏感字段。
//	@Tags			校友本人资料
//	@Produce		json
//	@Success		200	{object}	response.Body{data=dto.AlumniDetail}
//	@Failure		401	{object}	response.Body	"未登录或用户不存在"
//	@Failure		403	{object}	response.Body	"非校友账号、账号已停用，或当前账号未绑定校友档案"
//	@Failure		404	{object}	response.Body	"对应校友不存在"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/alumni/me [get]
func (h *AlumniHandler) Me(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.alumni.GetMe(c.Request.Context(), *access)
	if err == nil {
		response.Success(c, result)
		return
	}

	h.writeMeError(c, err)
}

// UpdateMe 修改当前登录校友的本人资料。
//
//	@Summary		修改本人资料
//	@Description	仅校友本人可调用。传入的字段才会被修改，未传字段保持原值。
//	@Tags			校友本人资料
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.AlumniProfileUpdateRequest	true	"可自行维护的字段"
//	@Success		200		{object}	response.Body{data=dto.AlumniDetail}
//	@Failure		401		{object}	response.Body	"未登录或用户不存在"
//	@Failure		403		{object}	response.Body	"非校友账号、账号已停用，或当前账号未绑定校友档案"
//	@Failure		404		{object}	response.Body	"对应校友不存在"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/alumni/me [put]
func (h *AlumniHandler) UpdateMe(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.AlumniProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.UpdateMe(c.Request.Context(), *access, req)
	if err == nil {
		response.Success(c, result)
		return
	}

	h.writeMeError(c, err)
}

func (h *AlumniHandler) writeMeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrUserNotFound):
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
	case errors.Is(err, common.ErrAccountDisabled):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "account is disabled")
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "仅校友账号可访问本人资料")
	case errors.Is(err, common.ErrAlumniProfileUnbound):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "当前账号未绑定校友档案")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
