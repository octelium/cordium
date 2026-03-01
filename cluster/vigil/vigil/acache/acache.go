/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package acache

import (
	"fmt"
	"time"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/apiutils/ucorev1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
)

type Cache struct {
	// db *bbolt.DB
	c *cache.Cache
}

func NewCache() (*Cache, error) {

	/*
		filename := fmt.Sprintf("/tmp/acache-%s.db", utilrand.GetRandomStringLowercase(6))
		db, err := bbolt.Open(filename, 0600, nil)
		if err != nil {
			return nil, err
		}

		if err := db.Update(func(tx *bbolt.Tx) error {

			bucketNames := []string{
				ucordiumv1.KindWorkspace,
				ucordiumv1.KindSpace,
				ucordiumv1.KindMembership,
				ucorev1.KindSession,
			}

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
	*/

	return &Cache{
		// db: db,
		c: cache.New(5*cache.NoExpiration, 10*time.Minute),
	}, nil
}

var ErrNotFound = errors.New("Resource not found")

func (c *Cache) setResource(key string, obj umetav1.ResourceObjectI, kind string) error {

	c.c.Set(fmt.Sprintf("%s:%s", kind, key), obj, cache.NoExpiration)
	/*
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
	*/

	return nil
}

func (c *Cache) getResource(kind string, key string, newObjFn func(kind string) (umetav1.ResourceObjectI, error)) (umetav1.ResourceObjectI, error) {

	ret, ok := c.c.Get(fmt.Sprintf("%s:%s", kind, key))
	if !ok {
		return nil, ErrNotFound
	}
	/*
		rsc, err := newObjFn(kind)
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
	*/
	return ret.(umetav1.ResourceObjectI), nil
}

func (c *Cache) deleteResource(kind string, key string) error {

	c.c.Delete(fmt.Sprintf("%s:%s", kind, key))

	/*
		if err := c.db.Update(func(tx *bbolt.Tx) error {

			b := tx.Bucket([]byte(kind))

			err := b.Delete([]byte(key))

			return err

		}); err != nil {
			return err
		}
	*/
	return nil
}

func (c *Cache) GetWorkspace(workspaceName string) (*cordiumv1.Workspace, error) {
	ret, err := c.getResource(ucordiumv1.KindWorkspace, workspaceName, ucordiumv1.NewObject)
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
	ret, err := c.getResource(ucordiumv1.KindSpace, uid, ucordiumv1.NewObject)
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

func (c *Cache) GetSession(uid string) (*corev1.Session, error) {
	ret, err := c.getResource(ucorev1.KindSession, uid, ucorev1.NewObject)
	if err != nil {
		return nil, err
	}
	return ret.(*corev1.Session), nil
}

func (c *Cache) SetSession(sess *corev1.Session) error {
	return c.setResource(sess.Metadata.Uid, sess, ucorev1.KindSession)
}

func (c *Cache) DeleteSession(sess *corev1.Session) error {
	return c.deleteResource(ucorev1.KindSession, sess.Metadata.Uid)
}

func (c *Cache) GetMembership(spcRef, usrRef *metav1.ObjectReference) (*cordiumv1.Membership, error) {
	ret, err := c.getResource(ucordiumv1.KindMembership, workspacecommon.GetMembershipName(spcRef, usrRef), ucordiumv1.NewObject)
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
