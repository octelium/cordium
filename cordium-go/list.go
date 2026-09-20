/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cordium

import (
	"context"
	"iter"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
)

// ListOption narrows down, orders or paginates a list request. The options that
// do not apply to a given resource are ignored by it.
type ListOption func(*listConfig) error

type listConfig struct {
	page         uint32
	itemsPerPage uint32
	orderByType  metav1.CommonListOptions_OrderBy_Type
	orderByMode  metav1.CommonListOptions_OrderBy_Mode

	spaceRef    *metav1.ObjectReference
	templateRef *metav1.ObjectReference

	spaceType cordiumv1.Space_Status_Type
	spaceMode cordiumv1.ListSpaceOptions_Mode
}

func newListConfig(opts ...ListOption) (*listConfig, error) {
	ret := &listConfig{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(ret); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (l *listConfig) common() *metav1.CommonListOptions {
	ret := &metav1.CommonListOptions{
		Page:         l.page,
		ItemsPerPage: l.itemsPerPage,
	}
	if l.orderByType != metav1.CommonListOptions_OrderBy_TYPE_UNSET ||
		l.orderByMode != metav1.CommonListOptions_OrderBy_MODE_UNSET {
		ret.OrderBy = &metav1.CommonListOptions_OrderBy{
			Type: l.orderByType,
			Mode: l.orderByMode,
		}
	}
	return ret
}

// WithPage requests a specific page of the list. Pages start at zero.
func WithPage(page uint32) ListOption {
	return func(l *listConfig) error {
		l.page = page
		return nil
	}
}

// WithItemsPerPage sets the page size of the list.
func WithItemsPerPage(items uint32) ListOption {
	return func(l *listConfig) error {
		l.itemsPerPage = items
		return nil
	}
}

// OrderByName orders the list by the objects' names.
func OrderByName() ListOption {
	return func(l *listConfig) error {
		l.orderByType = metav1.CommonListOptions_OrderBy_NAME
		return nil
	}
}

// OrderByCreatedAt orders the list by the objects' creation dates.
func OrderByCreatedAt() ListOption {
	return func(l *listConfig) error {
		l.orderByType = metav1.CommonListOptions_OrderBy_CREATED_AT
		return nil
	}
}

// Ascending orders the list in an ascending order.
func Ascending() ListOption {
	return func(l *listConfig) error {
		l.orderByMode = metav1.CommonListOptions_OrderBy_ASC
		return nil
	}
}

// Descending orders the list in a descending order.
func Descending() ListOption {
	return func(l *listConfig) error {
		l.orderByMode = metav1.CommonListOptions_OrderBy_DESC
		return nil
	}
}

// InSpace restricts the list to the resources that belong to a Space.
func InSpace(name string) ListOption {
	return func(l *listConfig) error {
		if name == "" {
			return invalidArgumentf("empty Space name")
		}
		l.spaceRef = &metav1.ObjectReference{Name: name}
		return nil
	}
}

// OfTemplate restricts the list to the Workspaces that were created from a
// Template.
func OfTemplate(name string) ListOption {
	return func(l *listConfig) error {
		if name == "" {
			return invalidArgumentf("empty Template name")
		}
		l.templateRef = &metav1.ObjectReference{Name: name}
		return nil
	}
}

// OwnedSpaces lists only the Spaces that were created by the calling User. It
// is the default mode of [SpaceClient.List].
func OwnedSpaces() ListOption {
	return func(l *listConfig) error {
		l.spaceMode = cordiumv1.ListSpaceOptions_MODE_CREATED_BY
		return nil
	}
}

// MemberSpaces lists the Spaces in which the calling User has a Membership.
func MemberSpaces() ListOption {
	return func(l *listConfig) error {
		l.spaceMode = cordiumv1.ListSpaceOptions_MODE_MEMBER
		return nil
	}
}

// UserSpaces lists only the personal Spaces.
func UserSpaces() ListOption {
	return func(l *listConfig) error {
		l.spaceType = cordiumv1.Space_Status_USER
		return nil
	}
}

// OrganizationSpaces lists only the shared, multi-Member Spaces.
func OrganizationSpaces() ListOption {
	return func(l *listConfig) error {
		l.spaceType = cordiumv1.Space_Status_ORGANIZATION
		return nil
	}
}

// PageInfo is the pagination information of a list response.
type PageInfo struct {
	// Page is the returned page number. Pages start at zero.
	Page uint32
	// ItemsPerPage is the size of the returned page.
	ItemsPerPage uint32
	// TotalCount is the total number of the items that can be obtained.
	TotalCount uint32
	// HasMore reports whether a next page is available.
	HasMore bool
}

func pageInfoFrom(meta *metav1.ListResponseMeta) PageInfo {
	return PageInfo{
		Page:         meta.GetPage(),
		ItemsPerPage: meta.GetItemsPerPage(),
		TotalCount:   meta.GetTotalCount(),
		HasMore:      meta.GetHasMore(),
	}
}

// defaultPageSize is the page size that the ListAll helpers use when the caller
// does not set one.
const defaultPageSize = 100

// allPages turns a paginated List method into an iterator that walks every
// page. The caller's options are applied on top of the default page size, and
// the page number is always the iterator's own.
func allPages[T any](ctx context.Context, opts []ListOption,
	fetch func(context.Context, ...ListOption) ([]T, PageInfo, error)) iter.Seq2[T, error] {

	return func(yield func(T, error) bool) {
		var zero T

		for page := uint32(0); ; page++ {
			pageOpts := make([]ListOption, 0, len(opts)+2)
			pageOpts = append(pageOpts, WithItemsPerPage(defaultPageSize))
			pageOpts = append(pageOpts, opts...)
			pageOpts = append(pageOpts, WithPage(page))

			items, info, err := fetch(ctx, pageOpts...)
			if err != nil {
				yield(zero, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if !info.HasMore || len(items) == 0 {
				return
			}
		}
	}
}
