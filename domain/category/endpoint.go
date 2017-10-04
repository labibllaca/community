// Copyright 2016 Documize Inc. <legal@documize.com>. All rights reserved.
//
// This software (Documize Community Edition) is licensed under
// GNU AGPL v3 http://www.gnu.org/licenses/agpl-3.0.en.html
//
// You can operate outside the AGPL restrictions by purchasing
// Documize Enterprise Edition and obtaining a commercial license
// by contacting <sales@documize.com>.
//
// https://documize.com

// Package category handles API calls and persistence for categories.
// Categories sub-divide spaces.
package category

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/documize/community/core/env"
	"github.com/documize/community/core/request"
	"github.com/documize/community/core/response"
	"github.com/documize/community/core/uniqueid"
	"github.com/documize/community/domain"
	"github.com/documize/community/domain/permission"
	"github.com/documize/community/model/audit"
	"github.com/documize/community/model/category"
	pm "github.com/documize/community/model/permission"
)

// Handler contains the runtime information such as logging and database.
type Handler struct {
	Runtime *env.Runtime
	Store   *domain.Store
}

// Add saves space category.
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	method := "category.add"
	ctx := domain.GetRequestContext(r)

	if !ctx.Authenticated {
		response.WriteForbiddenError(w)
		return
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		response.WriteBadRequestError(w, method, "body")
		h.Runtime.Log.Error(method, err)
		return
	}

	var cat category.Category
	err = json.Unmarshal(body, &cat)
	if err != nil {
		response.WriteBadRequestError(w, method, "category")
		h.Runtime.Log.Error(method, err)
		return
	}

	cat.RefID = uniqueid.Generate()
	cat.OrgID = ctx.OrgID

	ctx.Transaction, err = h.Runtime.Db.Beginx()
	if err != nil {
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	err = h.Store.Category.Add(ctx, cat)
	if err != nil {
		ctx.Transaction.Rollback()
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	h.Store.Audit.Record(ctx, audit.EventTypeCategoryAdd)

	ctx.Transaction.Commit()

	cat, err = h.Store.Category.Get(ctx, cat.RefID)
	if err != nil {
		response.WriteServerError(w, method, err)
		return
	}

	response.WriteJSON(w, cat)
}

// Get returns categories visible to user within a space.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	method := "category.get"
	ctx := domain.GetRequestContext(r)

	spaceID := request.Param(r, "spaceID")
	if len(spaceID) == 0 {
		response.WriteMissingDataError(w, method, "spaceID")
		return
	}

	ok := permission.HasPermission(ctx, *h.Store, spaceID,
		pm.SpaceManage, pm.SpaceOwner, pm.SpaceView)
	if !ok {
		response.WriteForbiddenError(w)
		return
	}

	cat, err := h.Store.Category.GetBySpace(ctx, spaceID)
	if err != nil && err != sql.ErrNoRows {
		response.WriteServerError(w, method, err)
		return
	}

	if len(cat) == 0 {
		cat = []category.Category{}
	}

	response.WriteJSON(w, cat)
}

// GetAll returns categories within a space, disregarding permissions.
// Used in admin screens, lists, functions.
func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	method := "category.getAll"
	ctx := domain.GetRequestContext(r)

	spaceID := request.Param(r, "spaceID")
	if len(spaceID) == 0 {
		response.WriteMissingDataError(w, method, "spaceID")
		return
	}

	cat, err := h.Store.Category.GetAllBySpace(ctx, spaceID)
	if err != nil && err != sql.ErrNoRows {
		response.WriteServerError(w, method, err)
		return
	}

	if len(cat) == 0 {
		cat = []category.Category{}
	}

	response.WriteJSON(w, cat)
}

// Update saves existing space category.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	method := "category.update"
	ctx := domain.GetRequestContext(r)

	categoryID := request.Param(r, "categoryID")
	if len(categoryID) == 0 {
		response.WriteMissingDataError(w, method, "categoryID")
		return
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		response.WriteBadRequestError(w, method, "body")
		h.Runtime.Log.Error(method, err)
		return
	}

	var cat category.Category
	err = json.Unmarshal(body, &cat)
	if err != nil {
		response.WriteBadRequestError(w, method, "category")
		h.Runtime.Log.Error(method, err)
		return
	}

	cat.OrgID = ctx.OrgID
	cat.RefID = categoryID

	ok := permission.HasPermission(ctx, *h.Store, cat.LabelID, pm.SpaceManage, pm.SpaceOwner)
	if !ok || !ctx.Authenticated {
		response.WriteForbiddenError(w)
		return
	}

	ctx.Transaction, err = h.Runtime.Db.Beginx()
	if err != nil {
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	err = h.Store.Category.Update(ctx, cat)
	if err != nil {
		ctx.Transaction.Rollback()
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	h.Store.Audit.Record(ctx, audit.EventTypeCategoryUpdate)

	ctx.Transaction.Commit()

	cat, err = h.Store.Category.Get(ctx, cat.RefID)
	if err != nil {
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	response.WriteJSON(w, cat)
}

// Delete removes category and associated member records.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	method := "category.delete"
	ctx := domain.GetRequestContext(r)

	catID := request.Param(r, "categoryID")
	if len(catID) == 0 {
		response.WriteMissingDataError(w, method, "categoryID")
		return
	}

	cat, err := h.Store.Category.Get(ctx, catID)
	if err != nil {
		response.WriteServerError(w, method, err)
		return
	}

	ok := permission.HasPermission(ctx, *h.Store, cat.LabelID, pm.SpaceManage, pm.SpaceOwner)
	if !ok || !ctx.Authenticated {
		response.WriteForbiddenError(w)
		return
	}

	ctx.Transaction, err = h.Runtime.Db.Beginx()
	if err != nil {
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	// remove category members
	_, err = h.Store.Category.RemoveCategoryMembership(ctx, cat.RefID)
	if err != nil {
		ctx.Transaction.Rollback()
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	// remove category permissions
	_, err = h.Store.Permission.DeleteCategoryPermissions(ctx, cat.RefID)
	if err != nil {
		ctx.Transaction.Rollback()
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	// remove category
	_, err = h.Store.Category.Delete(ctx, cat.RefID)
	if err != nil {
		ctx.Transaction.Rollback()
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	h.Store.Audit.Record(ctx, audit.EventTypeCategoryDelete)

	ctx.Transaction.Commit()

	response.WriteEmpty(w)
}

// GetSummary returns number of documents and users for space categories.
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	method := "category.GetSummary"
	ctx := domain.GetRequestContext(r)

	spaceID := request.Param(r, "spaceID")
	if len(spaceID) == 0 {
		response.WriteMissingDataError(w, method, "spaceID")
		return
	}

	ok := permission.HasPermission(ctx, *h.Store, spaceID, pm.SpaceManage, pm.SpaceOwner, pm.SpaceView)
	if !ok {
		response.WriteForbiddenError(w)
		return
	}

	s, err := h.Store.Category.GetSpaceCategorySummary(ctx, spaceID)
	if err != nil {
		h.Runtime.Log.Error("get space category summary failed", err)
		response.WriteServerError(w, method, err)
		return
	}

	if len(s) == 0 {
		s = []category.SummaryModel{}
	}

	response.WriteJSON(w, s)
}

// SetDocumentCategoryMembership will link/unlink document from categories (query string switch mode=link or mode=unlink).
func (h *Handler) SetDocumentCategoryMembership(w http.ResponseWriter, r *http.Request) {
	method := "category.addMember"
	ctx := domain.GetRequestContext(r)

	mode := request.Query(r, "mode")
	if len(mode) == 0 {
		response.WriteMissingDataError(w, method, "mode")
		return
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		response.WriteBadRequestError(w, method, "body")
		h.Runtime.Log.Error(method, err)
		return
	}

	var cats []category.Member
	err = json.Unmarshal(body, &cats)
	if err != nil {
		response.WriteBadRequestError(w, method, "category")
		h.Runtime.Log.Error(method, err)
		return
	}

	if len(cats) == 0 {
		response.WriteEmpty(w)
		return
	}

	if !permission.HasPermission(ctx, *h.Store, cats[0].LabelID, pm.DocumentAdd, pm.DocumentEdit) {
		response.WriteForbiddenError(w)
		return
	}

	ctx.Transaction, err = h.Runtime.Db.Beginx()
	if err != nil {
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error(method, err)
		return
	}

	for _, c := range cats {
		if mode == "link" {
			c.OrgID = ctx.OrgID
			c.RefID = uniqueid.Generate()
			_, err = h.Store.Category.DisassociateDocument(ctx, c.CategoryID, c.DocumentID)
			err = h.Store.Category.AssociateDocument(ctx, c)
		} else {
			_, err = h.Store.Category.DisassociateDocument(ctx, c.CategoryID, c.DocumentID)
		}

		if err != nil {
			ctx.Transaction.Rollback()
			response.WriteServerError(w, method, err)
			h.Runtime.Log.Error(method, err)
			return
		}
	}

	h.Store.Audit.Record(ctx, audit.EventTypeCategoryLink)

	ctx.Transaction.Commit()

	response.WriteEmpty(w)
}

// GetDocumentCategoryMembership returns categories associated with given document.
func (h *Handler) GetDocumentCategoryMembership(w http.ResponseWriter, r *http.Request) {
	method := "category.GetDocumentCategoryMembership"
	ctx := domain.GetRequestContext(r)

	documentID := request.Param(r, "documentID")
	if len(documentID) == 0 {
		response.WriteMissingDataError(w, method, "documentID")
		return
	}

	doc, err := h.Store.Document.Get(ctx, documentID)
	if err != nil {
		response.WriteServerError(w, method, err)
		h.Runtime.Log.Error("no document for category", err)
		return
	}

	if !permission.HasPermission(ctx, *h.Store, doc.LabelID, pm.SpaceView, pm.DocumentAdd, pm.DocumentEdit) {
		response.WriteForbiddenError(w)
		return
	}

	cat, err := h.Store.Category.GetDocumentCategoryMembership(ctx, doc.RefID)
	if err != nil && err != sql.ErrNoRows {
		h.Runtime.Log.Error("get document category membership", err)
		response.WriteServerError(w, method, err)
		return
	}

	if len(cat) == 0 {
		cat = []category.Category{}
	}

	response.WriteJSON(w, cat)
}

// GetSpaceCategoryMembers returns category/document associations within space.
func (h *Handler) GetSpaceCategoryMembers(w http.ResponseWriter, r *http.Request) {
	method := "category.GetSpaceCategoryMembers"
	ctx := domain.GetRequestContext(r)

	spaceID := request.Param(r, "spaceID")
	if len(spaceID) == 0 {
		response.WriteMissingDataError(w, method, "spaceID")
		return
	}

	if !permission.HasPermission(ctx, *h.Store, spaceID, pm.SpaceView) {
		response.WriteForbiddenError(w)
		return
	}

	cat, err := h.Store.Category.GetSpaceCategoryMembership(ctx, spaceID)
	if err != nil && err != sql.ErrNoRows {
		h.Runtime.Log.Error("get document category membership for space", err)
		response.WriteServerError(w, method, err)
		return
	}

	if len(cat) == 0 {
		cat = []category.Member{}
	}

	response.WriteJSON(w, cat)
}
