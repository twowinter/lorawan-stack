// Copyright © 2018 The Things Network Foundation, The Things Industries B.V.
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

package identityserver

import (
	"context"

	"github.com/gogo/protobuf/types"
	"github.com/jinzhu/gorm"
	"go.thethings.network/lorawan-stack/pkg/auth"
	"go.thethings.network/lorawan-stack/pkg/auth/rights"
	"go.thethings.network/lorawan-stack/pkg/identityserver/store"
	"go.thethings.network/lorawan-stack/pkg/ttnpb"
	"go.thethings.network/lorawan-stack/pkg/unique"
)

func (is *IdentityServer) listApplicationRights(ctx context.Context, ids *ttnpb.ApplicationIdentifiers) (*ttnpb.Rights, error) {
	rights, ok := rights.FromContext(ctx)
	if !ok {
		return &ttnpb.Rights{}, nil
	}
	appRights, ok := rights.ApplicationRights[unique.ID(ctx, ids)]
	if !ok || appRights == nil {
		return &ttnpb.Rights{}, nil
	}
	return appRights, nil
}

func (is *IdentityServer) createApplicationAPIKey(ctx context.Context, req *ttnpb.CreateApplicationAPIKeyRequest) (key *ttnpb.APIKey, err error) {
	err = rights.RequireApplication(ctx, req.ApplicationIdentifiers, ttnpb.RIGHT_APPLICATION_SETTINGS_API_KEYS)
	if err != nil {
		return nil, err
	}
	err = rights.RequireApplication(ctx, req.ApplicationIdentifiers, req.Rights...)
	if err != nil {
		return nil, err
	}
	id, err := auth.GenerateID(ctx)
	if err != nil {
		return nil, err
	}
	token, err := auth.APIKey.Generate(ctx, id)
	if err != nil {
		return nil, err
	}
	key = &ttnpb.APIKey{
		ID:     id,
		Key:    token,
		Name:   req.Name,
		Rights: req.Rights,
	}
	err = is.withDatabase(ctx, func(db *gorm.DB) (err error) {
		keyStore := store.GetAPIKeyStore(db)
		err = keyStore.CreateAPIKey(ctx, req.ApplicationIdentifiers.EntityIdentifiers(), key)
		return err
	})
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (is *IdentityServer) listApplicationAPIKeys(ctx context.Context, ids *ttnpb.ApplicationIdentifiers) (keys *ttnpb.APIKeys, err error) {
	err = rights.RequireApplication(ctx, *ids, ttnpb.RIGHT_APPLICATION_SETTINGS_API_KEYS)
	if err != nil {
		return nil, err
	}
	keys = new(ttnpb.APIKeys)
	err = is.withDatabase(ctx, func(db *gorm.DB) (err error) {
		keyStore := store.GetAPIKeyStore(db)
		keys.APIKeys, err = keyStore.FindAPIKeys(ctx, ids.EntityIdentifiers())
		return err
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (is *IdentityServer) updateApplicationAPIKey(ctx context.Context, req *ttnpb.UpdateApplicationAPIKeyRequest) (key *ttnpb.APIKey, err error) {
	err = rights.RequireApplication(ctx, req.ApplicationIdentifiers, ttnpb.RIGHT_APPLICATION_SETTINGS_API_KEYS)
	if err != nil {
		return nil, err
	}
	err = rights.RequireApplication(ctx, req.ApplicationIdentifiers, req.Rights...)
	if err != nil {
		return nil, err
	}
	err = is.withDatabase(ctx, func(db *gorm.DB) (err error) {
		keyStore := store.GetAPIKeyStore(db)
		key, err = keyStore.UpdateAPIKey(ctx, req.ApplicationIdentifiers.EntityIdentifiers(), &req.APIKey)
		return err
	})
	if err != nil {
		return nil, err
	}
	if key == nil {
		return &ttnpb.APIKey{}, nil
	}
	return key, nil
}

func (is *IdentityServer) setApplicationCollaborator(ctx context.Context, req *ttnpb.SetApplicationCollaboratorRequest) (*types.Empty, error) {
	err := rights.RequireApplication(ctx, req.ApplicationIdentifiers, ttnpb.RIGHT_APPLICATION_SETTINGS_COLLABORATORS)
	if err != nil {
		return nil, err
	}
	err = rights.RequireApplication(ctx, req.ApplicationIdentifiers, req.Collaborator.Rights...)
	if err != nil {
		return nil, err
	}
	err = is.withDatabase(ctx, func(db *gorm.DB) (err error) {
		memberStore := store.GetMembershipStore(db)
		err = memberStore.SetMember(ctx, &req.Collaborator.OrganizationOrUserIdentifiers, req.ApplicationIdentifiers.EntityIdentifiers(), ttnpb.RightsFrom(req.Collaborator.Rights...))
		return err
	})
	if err != nil {
		return nil, err
	}
	return ttnpb.Empty, nil
}

func (is *IdentityServer) listApplicationCollaborators(ctx context.Context, ids *ttnpb.ApplicationIdentifiers) (collaborators *ttnpb.Collaborators, err error) {
	err = rights.RequireApplication(ctx, *ids, ttnpb.RIGHT_APPLICATION_SETTINGS_COLLABORATORS)
	if err != nil {
		return nil, err
	}
	err = is.withDatabase(ctx, func(db *gorm.DB) (err error) {
		memberStore := store.GetMembershipStore(db)
		memberRights, err := memberStore.FindMembers(ctx, ids.EntityIdentifiers())
		if err != nil {
			return err
		}
		collaborators = new(ttnpb.Collaborators)
		for member, rights := range memberRights {
			collaborators.Collaborators = append(collaborators.Collaborators, &ttnpb.Collaborator{
				OrganizationOrUserIdentifiers: *member,
				Rights:                        rights.GetRights(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return collaborators, nil
}

type applicationAccess struct {
	*IdentityServer
}

func (aa *applicationAccess) ListRights(ctx context.Context, req *ttnpb.ApplicationIdentifiers) (*ttnpb.Rights, error) {
	return aa.listApplicationRights(ctx, req)
}
func (aa *applicationAccess) CreateAPIKey(ctx context.Context, req *ttnpb.CreateApplicationAPIKeyRequest) (*ttnpb.APIKey, error) {
	return aa.createApplicationAPIKey(ctx, req)
}
func (aa *applicationAccess) ListAPIKeys(ctx context.Context, req *ttnpb.ApplicationIdentifiers) (*ttnpb.APIKeys, error) {
	return aa.listApplicationAPIKeys(ctx, req)
}
func (aa *applicationAccess) UpdateAPIKey(ctx context.Context, req *ttnpb.UpdateApplicationAPIKeyRequest) (*ttnpb.APIKey, error) {
	return aa.updateApplicationAPIKey(ctx, req)
}
func (aa *applicationAccess) SetCollaborator(ctx context.Context, req *ttnpb.SetApplicationCollaboratorRequest) (*types.Empty, error) {
	return aa.setApplicationCollaborator(ctx, req)
}
func (aa *applicationAccess) ListCollaborators(ctx context.Context, req *ttnpb.ApplicationIdentifiers) (*ttnpb.Collaborators, error) {
	return aa.listApplicationCollaborators(ctx, req)
}
