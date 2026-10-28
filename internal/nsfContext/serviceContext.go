package nsfContext

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"nsf/internal/db"
	"nsf/logger"
)

const nfKeyPrefix = "nsf:nf:"

type serviceContextIE struct {
	db.DbIf
	*logger.NsfLogger
}

type serviceContext struct {
	db.DbIf
	*logger.NsfLogger
	mu sync.RWMutex
}

type nfRecord struct {
	NfName     string   `json:"nf_name"`
	SbiAddress string   `json:"sbi_address"`
	SbiPort    int      `json:"sbi_port"`
	Services   []string `json:"services"`
}

func newServiceContext(ie *serviceContextIE) *serviceContext {
	return &serviceContext{DbIf: ie.DbIf, NsfLogger: ie.NsfLogger}
}

// The database owns the durable state and is closed by dbContext.
// Releasing a service context must not erase registrations needed on restart.
func (s *serviceContext) release() {
	s.SvcLog.Infoln("serviceContext released")
}

func (s *serviceContext) registerServices(nfName, sbiAddr string, sbiPort int, services []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := nfRecord{NfName: nfName, SbiAddress: sbiAddr, SbiPort: sbiPort}
	key := nfKeyPrefix + nfName
	exists, err := s.Exist(key)
	if err != nil {
		return err
	}
	if exists {
		value, loadErr := s.Load(key)
		if loadErr != nil {
			return loadErr
		}
		var previous nfRecord
		if err := json.Unmarshal([]byte(value), &previous); err != nil {
			return err
		}
		record.Services = previous.Services
	}
	seen := make(map[string]bool)
	for _, service := range record.Services {
		seen[service] = true
	}
	for _, service := range services {
		if !seen[service] {
			record.Services = append(record.Services, service)
			seen[service] = true
		}
	}
	value, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.Save(key, string(value))
}

func (s *serviceContext) deregisterServices(nfName string, services []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := nfKeyPrefix + nfName
	exists, err := s.Exist(key)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	value, err := s.Load(key)
	if err != nil {
		return err
	}
	var record nfRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return err
	}
	removed := make(map[string]bool, len(services))
	for _, service := range services {
		removed[service] = true
	}
	remaining := make([]string, 0, len(record.Services))
	for _, service := range record.Services {
		if !removed[service] {
			remaining = append(remaining, service)
		}
	}
	if len(remaining) == 0 {
		return s.Remove(key)
	}
	record.Services = remaining
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.Save(key, string(encoded))
}

func (s *serviceContext) discoverService(service string) (string, string, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records, err := s.LoadAll()
	if err != nil {
		return "", "", 0, err
	}
	keys := make([]string, 0, len(records))
	for key := range records {
		if strings.HasPrefix(key, nfKeyPrefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		var record nfRecord
		if err := json.Unmarshal([]byte(records[key]), &record); err != nil {
			return "", "", 0, fmt.Errorf("decode registration %s: %w", key, err)
		}
		for _, registered := range record.Services {
			if registered == service {
				return record.NfName, record.SbiAddress, record.SbiPort, nil
			}
		}
	}
	return "", "", 0, fmt.Errorf("service %q not found", service)
}
