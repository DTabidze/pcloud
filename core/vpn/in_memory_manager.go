package vpn

import (
	"errors"
	"fmt"
	"sync"

	"github.com/giolekva/pcloud/core/vpn/types"
)

func errorDeviceNotFound(pubKey types.PublicKey) error {
	return fmt.Errorf("Device not found: %s", pubKey)
}

type InMemoryManager struct {
	lock         sync.Mutex
	devices      []*types.DeviceInfo
	keyToDevices map[types.PublicKey]*types.DeviceInfo
	callbacks    map[types.PublicKey][]NetworkMapChangeCallback
	ipm          IPManager
}

func NewInMemoryManager(ipm IPManager) *InMemoryManager {
	return &InMemoryManager{
		devices:      make([]*types.DeviceInfo, 0),
		keyToDevices: make(map[types.PublicKey]*types.DeviceInfo),
		callbacks:    make(map[types.PublicKey][]NetworkMapChangeCallback),
		ipm:          ipm,
	}
}

func (m *InMemoryManager) RegisterDevice(d types.DeviceInfo) (*types.NetworkMap, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.keyToDevices[d.PublicKey]; ok {
		return nil, errors.New(fmt.Sprintf("Device with given public key is already registered: %s", d.PublicKey))
	}
	if _, err := m.ipm.New(d.PublicKey); err != nil {
		return nil, err
	}
	m.keyToDevices[d.PublicKey] = &d
	m.devices = append(m.devices, &d)
	m.callbacks[d.PublicKey] = make([]NetworkMapChangeCallback, 0)
	ret := m.genNetworkMap(&d)
	// TODO(giolekva): run this in a goroutine
	for _, peer := range m.devices {
		if peer.PublicKey != d.PublicKey {
			netMap := m.genNetworkMap(peer)
			for _, cb := range m.callbacks[peer.PublicKey] {
				cb(netMap)
			}
		}
	}
	return ret, nil
}

func (m *InMemoryManager) RemoveDevice(pubKey types.PublicKey) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.keyToDevices[pubKey]; !ok {
		return errorDeviceNotFound(pubKey)
	}
	delete(m.keyToDevices, pubKey) // TODO(giolekva): maybe mark as deleted?
	for i, peer := range m.devices {
		if peer.PublicKey == pubKey {
			m.devices[i] = m.devices[len(m.devices)-1]
			m.devices = m.devices[:len(m.devices)-1]
		}
	}
	for _, peer := range m.devices {
		netMap := m.genNetworkMap(peer)
		for _, cb := range m.callbacks[peer.PublicKey] {
			cb(netMap)
		}
	}
	return nil
}

func (m *InMemoryManager) GetNetworkMap(pubKey types.PublicKey) (*types.NetworkMap, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if d, ok := m.keyToDevices[pubKey]; ok {
		return m.genNetworkMap(d), nil
	}
	return nil, errorDeviceNotFound(pubKey)
}

func (m *InMemoryManager) AddNetworkMapChangeCallback(pubKey types.PublicKey, cb NetworkMapChangeCallback) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.keyToDevices[pubKey]; ok {
		m.callbacks[pubKey] = append(m.callbacks[pubKey], cb)
	}
	return errorDeviceNotFound(pubKey)
}

func (m *InMemoryManager) genNetworkMap(d *types.DeviceInfo) *types.NetworkMap {
	vpnIP, err := m.ipm.Get(d.PublicKey)
	// NOTE(giolekva): Should not happen as devices must have been already registered and assigned IP address.
	// Maybe should return error anyways instead of panic?
	if err != nil {
		panic(err)
	}
	ret := types.NetworkMap{
		Self: types.Node{
			PublicKey:     d.PublicKey,
			DiscoKey:      d.DiscoKey,
			DiscoEndpoint: d.DiscoKey.Endpoint(),
			IPPort:        d.IPPort,
			VPNIP:         vpnIP,
		},
	}
	for _, peer := range m.devices {
		if d.PublicKey == peer.PublicKey {
			continue
		}
		vpnIP, err := m.ipm.Get(peer.PublicKey)
		if err != nil {
			panic(err)
		}
		ret.Peers = append(ret.Peers, types.Node{
			PublicKey:     peer.PublicKey,
			DiscoKey:      peer.DiscoKey,
			DiscoEndpoint: peer.DiscoKey.Endpoint(),
			IPPort:        peer.IPPort,
			VPNIP:         vpnIP,
		})
	}
	return &ret
}
