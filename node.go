package substrate

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v3/scale"
	"github.com/centrifuge/go-substrate-rpc-client/v3/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Resources type
type Resources struct {
	HRU types.U64
	SRU types.U64
	CRU types.U64
	MRU types.U64
}

// Location type
type Location struct {
	Longitude string
	Latitude  string
}

// Role type
type Role struct {
	IsNode    bool
	IsGateway bool
}

// Decode implementation for the enum type
func (r *Role) Decode(decoder scale.Decoder) error {
	b, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}

	switch b {
	case 0:
		r.IsNode = true
	case 1:
		r.IsGateway = true
	default:
		return fmt.Errorf("unknown CertificateType value")
	}

	return nil
}

// Encode implementation
func (r Role) Encode(encoder scale.Encoder) (err error) {
	if r.IsNode {
		err = encoder.PushByte(0)
	} else if r.IsGateway {
		err = encoder.PushByte(1)
	}

	return
}

// PublicConfig type
type PublicConfig struct {
	IPv4   string
	IPv6   string
	GWv4   string
	GWv6   string
	Domain string
}

// OptionPublicConfig type
type OptionPublicConfig struct {
	HasValue bool
	AsValue  PublicConfig
}

// Encode implementation
func (m OptionPublicConfig) Encode(encoder scale.Encoder) (err error) {
	var i byte
	if m.HasValue {
		i = 1
	}
	err = encoder.PushByte(i)
	if err != nil {
		return err
	}

	if m.HasValue {
		err = encoder.Encode(m.AsValue)
	}

	return
}

// Decode implementation
func (m *OptionPublicConfig) Decode(decoder scale.Decoder) (err error) {
	var i byte
	if err := decoder.Decode(&i); err != nil {
		return err
	}

	switch i {
	case 0:
		return nil
	case 1:
		m.HasValue = true
		return decoder.Decode(&m.AsValue)
	default:
		return fmt.Errorf("unknown value for Option")
	}
}

type Interface struct {
	Name string
	Mac  string
	IPs  []string
}

// Node type
type Node struct {
	Versioned
	ID                types.U32
	FarmID            types.U32
	TwinID            types.U32
	Resources         Resources
	Location          Location
	Country           string
	City              string
	PublicConfig      OptionPublicConfig
	Created           types.U64
	FarmingPolicy     types.U32
	Interfaces        []Interface
	CertificationType CertificationType
}

//GetNodeByTwinID gets a node by twin id
func (s *Substrate) GetNodeByTwinID(twin uint32) (uint32, error) {
	cl, meta, err := s.pool.Get()
	if err != nil {
		return 0, err
	}
	bytes, err := types.EncodeToBytes(twin)
	if err != nil {
		return 0, err
	}
	key, err := types.CreateStorageKey(meta, "TfgridModule", "NodeIdByTwinID", bytes, nil)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create substrate query key")
	}
	var id types.U32
	ok, err := cl.RPC.State.GetStorageLatest(key, &id)
	if err != nil {
		return 0, errors.Wrap(err, "failed to lookup entity")
	}

	if !ok || id == 0 {
		return 0, errors.Wrap(ErrNotFound, "node not found")
	}

	return uint32(id), nil
}

// GetNode with id
func (s *Substrate) GetNode(id uint32) (*Node, error) {
	cl, meta, err := s.pool.Get()
	if err != nil {
		return nil, err
	}

	bytes, err := types.EncodeToBytes(id)
	if err != nil {
		return nil, errors.Wrap(err, "substrate: encoding error building query arguments")
	}
	key, err := types.CreateStorageKey(meta, "TfgridModule", "Nodes", bytes, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create substrate query key")
	}

	return s.getNode(cl, key)
}

func (s *Substrate) getNode(cl Conn, key types.StorageKey) (*Node, error) {
	raw, err := cl.RPC.State.GetStorageRawLatest(key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to lookup entity")
	}

	if len(*raw) == 0 {
		return nil, errors.Wrap(ErrNotFound, "node not found")
	}

	version, err := s.getVersion(*raw)
	if err != nil {
		return nil, err
	}

	var node Node

	switch version {
	case 0:
		fallthrough
	case 1:
		fallthrough
	case 2:
		if err := types.DecodeFromBytes(*raw, &node); err != nil {
			return nil, errors.Wrap(err, "failed to load object")
		}
	default:
		return nil, ErrUnknownVersion
	}

	return &node, nil
}

// CreateNode creates a node, this ignores public_config since
// this is only setable by the farmer
func (s *Substrate) CreateNode(identity Identity, node Node) (uint32, error) {
	cl, meta, err := s.pool.Get()
	if err != nil {
		return 0, err
	}

	if node.TwinID == 0 {
		return 0, fmt.Errorf("twin id is required")
	}

	c, err := types.NewCall(meta, "TfgridModule.create_node",
		node.FarmID,
		node.Resources,
		node.Location,
		node.Country,
		node.City,
		node.Interfaces,
	)

	if err != nil {
		return 0, errors.Wrap(err, "failed to create call")
	}

	if _, err := s.Call(cl, meta, identity, c); err != nil {
		return 0, errors.Wrap(err, "failed to create node")
	}

	return s.GetNodeByTwinID(uint32(node.TwinID))

}

// UpdateNode updates a node, this ignores public_config and only keep the value
// set by the farmer
func (s *Substrate) UpdateNode(identity Identity, node Node) (uint32, error) {
	cl, meta, err := s.pool.Get()
	if err != nil {
		return 0, err
	}

	if node.TwinID == 0 {
		return 0, fmt.Errorf("twin id is required")
	}

	c, err := types.NewCall(meta, "TfgridModule.update_node",
		node.ID,
		node.FarmID,
		node.Resources,
		node.Location,
		node.Country,
		node.City,
		node.Interfaces,
	)

	if err != nil {
		return 0, errors.Wrap(err, "failed to create call")
	}

	if hash, err := s.Call(cl, meta, identity, c); err != nil {
		return 0, errors.Wrap(err, "failed to update node")
	} else {
		log.Debug().Str("hash", hash.Hex()).Msg("update call hash")
	}

	return s.GetNodeByTwinID(uint32(node.TwinID))
}

// UpdateNodeUptime updates the node uptime to given value
func (s *Substrate) UpdateNodeUptime(identity Identity, uptime uint64) error {
	cl, meta, err := s.pool.Get()
	if err != nil {
		return err
	}

	c, err := types.NewCall(meta, "TfgridModule.report_uptime", uptime)

	if err != nil {
		return errors.Wrap(err, "failed to create call")
	}

	if _, err := s.Call(cl, meta, identity, c); err != nil {
		return errors.Wrap(err, "failed to update node uptime")
	}

	return nil
}
