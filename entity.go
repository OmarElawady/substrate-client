package substrate

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/pkg/errors"
)

// Entity type
type Entity struct {
	Versioned
	ID      types.U32
	Name    string
	Account AccountID
	Country string
	City    string
}

// GetEntity gets a entity with ID
func (s *Substrate) GetEntity(id uint32) (*Entity, error) {
	cl, meta, err := s.getClient()
	if err != nil {
		return nil, err
	}

	bytes, err := types.EncodeToBytes(id)
	if err != nil {
		return nil, errors.Wrap(err, "substrate: encoding error building query arguments")
	}
	key, err := types.CreateStorageKey(meta, "TfgridModule", "Entities", bytes, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create substrate query key")
	}

	raw, err := cl.RPC.State.GetStorageRawLatest(key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to lookup entity")
	}

	if len(*raw) == 0 {
		return nil, errors.Wrap(ErrNotFound, "entity not found")
	}

	version, err := s.getVersion(*raw)
	if err != nil {
		return nil, err
	}

	var entity Entity

	switch version {
	case 1:
		if err := types.DecodeFromBytes(*raw, &entity); err != nil {
			return nil, errors.Wrap(err, "failed to load object")
		}
	default:
		return nil, ErrUnknownVersion
	}

	return &entity, nil
}
