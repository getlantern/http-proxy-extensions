package profilter

import (
	"sync"
	"sync/atomic"
)

type devicesMap map[uint64]map[string]bool

type DeviceRegistry struct {
	devicesPerUser atomic.Value
	womutex        sync.Mutex
}

func NewDeviceRegistry() *DeviceRegistry {
	return &DeviceRegistry{}
}

func (r *DeviceRegistry) SetUserDevice(userID uint64, device string) {
	r.womutex.Lock()
	defer r.womutex.Unlock()

	devicesPerUser := r.devicesPerUser.Load().(devicesMap)
	// Deep-copy the nested maps
	newDevicesPerUser := make(devicesMap)
	for k, v := range devicesPerUser {
		newDevices := make(map[string]bool)
		for k2, v2 := range v {
			newDevices[k2] = v2
		}
		newDevicesPerUser[k] = newDevices
	}
	newDevicesPerUser[userID][device] = true
	r.devicesPerUser.Store(newDevicesPerUser)
}

func (r *DeviceRegistry) GetUserDevices(userID uint64) map[string]bool {
	devices := r.devicesPerUser.Load().(devicesMap)
	return devices[userID]
}
