// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdserver

import (
	"github.com/coreos/etcd/auth"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type authApplierV3 struct {
	applierV3
	as   auth.AuthStore
	user string
}

func newAuthApplierV3(as auth.AuthStore, base applierV3) *authApplierV3 {
	return &authApplierV3{base, as, ""}
}

func (aa *authApplierV3) Apply(r *pb.InternalRaftRequest) *applyResult {
	var user string
	if r.Header != nil {
		// backward-compatible with pre-3.0 releases when internalRaftRequest
		// does not have header field
		user = r.Header.Username
	}
	if needAdminPermission(r) && !aa.as.IsAdminPermitted(user) {
		return &applyResult{err: auth.ErrPermissionDenied}
	}
	aa.user = user
	ret := aa.applierV3.Apply(r)
	aa.user = ""
	return ret
}

func (aa *authApplierV3) Put(txnID int64, r *pb.PutRequest) (*pb.PutResponse, error) {
	if !aa.as.IsPutPermitted(aa.user, r.Key) {
		return nil, auth.ErrPermissionDenied
	}
	return aa.applierV3.Put(txnID, r)
}

func (aa *authApplierV3) Range(txnID int64, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	if !aa.as.IsRangePermitted(aa.user, r.Key, r.RangeEnd) {
		return nil, auth.ErrPermissionDenied
	}
	return aa.applierV3.Range(txnID, r)
}

func (aa *authApplierV3) DeleteRange(txnID int64, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	if !aa.as.IsDeleteRangePermitted(aa.user, r.Key, r.RangeEnd) {
		return nil, auth.ErrPermissionDenied
	}
	return aa.applierV3.DeleteRange(txnID, r)
}

func (aa *authApplierV3) checkTxnReqsPermission(reqs []*pb.RequestOp) bool {
	for _, requ := range reqs {
		switch tv := requ.Request.(type) {
		case *pb.RequestOp_RequestRange:
			if tv.RequestRange == nil {
				continue
			}

			if !aa.as.IsRangePermitted(aa.user, tv.RequestRange.Key, tv.RequestRange.RangeEnd) {
				return false
			}

		case *pb.RequestOp_RequestPut:
			if tv.RequestPut == nil {
				continue
			}

			if !aa.as.IsPutPermitted(aa.user, tv.RequestPut.Key) {
				return false
			}

		case *pb.RequestOp_RequestDeleteRange:
			if tv.RequestDeleteRange == nil {
				continue
			}

			if !aa.as.IsDeleteRangePermitted(aa.user, tv.RequestDeleteRange.Key, tv.RequestDeleteRange.RangeEnd) {
				return false
			}
		}
	}

	return true
}

func (aa *authApplierV3) Txn(rt *pb.TxnRequest) (*pb.TxnResponse, error) {
	for _, c := range rt.Compare {
		if !aa.as.IsRangePermitted(aa.user, c.Key, nil) {
			return nil, auth.ErrPermissionDenied
		}
	}

	if !aa.checkTxnReqsPermission(rt.Success) {
		return nil, auth.ErrPermissionDenied
	}
	if !aa.checkTxnReqsPermission(rt.Failure) {
		return nil, auth.ErrPermissionDenied
	}

	return aa.applierV3.Txn(rt)
}

func needAdminPermission(r *pb.InternalRaftRequest) bool {
	switch {
	case r.AuthEnable != nil:
		return true
	case r.AuthDisable != nil:
		return true
	case r.AuthUserAdd != nil:
		return true
	case r.AuthUserDelete != nil:
		return true
	case r.AuthUserChangePassword != nil:
		return true
	case r.AuthUserGrantRole != nil:
		return true
	case r.AuthUserGet != nil:
		return true
	case r.AuthUserRevokeRole != nil:
		return true
	case r.AuthRoleAdd != nil:
		return true
	case r.AuthRoleGrantPermission != nil:
		return true
	case r.AuthRoleGet != nil:
		return true
	case r.AuthRoleRevokePermission != nil:
		return true
	case r.AuthRoleDelete != nil:
		return true
	default:
		return false
	}
}
