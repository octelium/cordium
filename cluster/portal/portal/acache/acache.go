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

package acache

import (
	"os"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

type Cache struct {
	db *bbolt.DB
}

const dbPath = "/tmp/octelium-acache.db"

func NewCache() (*Cache, error) {

	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}

	if err := db.Update(func(tx *bbolt.Tx) error {

		bucketNames := []string{ucordiumv1.KindWorkspace, ucordiumv1.KindSpace, ucordiumv1.KindMembership}

		for _, bucketName := range bucketNames {
			_, err = tx.CreateBucketIfNotExists([]byte(bucketName))
			if err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &Cache{
		db: db,
	}, nil
}

var ErrNotFound = errors.New("Resource not found")
var SessionNotFound = errors.New("Session not found")

func (c *Cache) setResource(key string, obj umetav1.ResourceObjectI, kind string) error {
	objBytes, err := pbutils.Marshal(obj)
	if err != nil {
		return err
	}

	if err := c.db.Update(func(tx *bbolt.Tx) error {

		b := tx.Bucket([]byte(kind))

		if key == "" {
			key = obj.GetMetadata().Uid
		}
		err := b.Put([]byte(key), objBytes)

		return err

	}); err != nil {
		return err
	}

	return nil
}

func (c *Cache) getResource(kind string, key string) (umetav1.ResourceObjectI, error) {
	rsc, err := ucordiumv1.NewObject(kind)
	if err != nil {
		return nil, err
	}

	if err := c.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kind))
		v := b.Get([]byte(key))
		if v == nil {
			return ErrNotFound
		}

		if err := pbutils.Unmarshal(v, rsc); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return rsc, nil
}

func (c *Cache) deleteResource(kind string, key string) error {
	if err := c.db.Update(func(tx *bbolt.Tx) error {

		b := tx.Bucket([]byte(kind))

		err := b.Delete([]byte(key))

		return err

	}); err != nil {
		return err
	}
	return nil
}

func (c *Cache) GetWorkspace(workspaceName string) (*cordiumv1.Workspace, error) {
	ret, err := c.getResource(ucordiumv1.KindWorkspace, workspaceName)
	if err != nil {
		return nil, err
	}
	return ret.(*cordiumv1.Workspace), nil
}

func (c *Cache) SetWorkspace(ws *cordiumv1.Workspace) error {
	return c.setResource(getKey(ws), ws, ucordiumv1.KindWorkspace)
}

func (c *Cache) DeleteWorkspace(ws *cordiumv1.Workspace) error {
	return c.deleteResource(ucordiumv1.KindWorkspace, getKey(ws))
}

func getKey(ws *cordiumv1.Workspace) string {
	return ws.Metadata.Name
}

func (c *Cache) GetSpace(uid string) (*cordiumv1.Space, error) {
	ret, err := c.getResource(ucordiumv1.KindSpace, uid)
	if err != nil {
		return nil, err
	}
	return ret.(*cordiumv1.Space), nil
}

func (c *Cache) SetSpace(spc *cordiumv1.Space) error {
	return c.setResource(spc.Metadata.Uid, spc, ucordiumv1.KindSpace)
}

func (c *Cache) DeleteSpace(spc *cordiumv1.Space) error {
	return c.deleteResource(ucordiumv1.KindSpace, spc.Metadata.Uid)
}

func (c *Cache) GetMembership(spcRef, usrRef *metav1.ObjectReference) (*cordiumv1.Membership, error) {
	ret, err := c.getResource(ucordiumv1.KindMembership, workspacecommon.GetMembershipName(spcRef, usrRef))
	if err != nil {
		return nil, err
	}
	return ret.(*cordiumv1.Membership), nil
}

func (c *Cache) SetMembership(itm *cordiumv1.Membership) error {
	return c.setResource(itm.Metadata.Name, itm, ucordiumv1.KindMembership)
}

func (c *Cache) DeleteMembership(itm *cordiumv1.Membership) error {
	return c.deleteResource(ucordiumv1.KindMembership, itm.Metadata.Name)
}
