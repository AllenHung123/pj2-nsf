package nsfContext

import (
	"encoding/json"
	"fmt"
	"nsf/internal/db"
	"nsf/logger"
	"strings"
)

type serviceInfo struct {
	NfName     string `json:"nf_name"`
	SbiAddress string `json:"sbi_address"`
	SbiPort    int    `json:"sbi_port"`
}

// TODO: You may need to add some fields
type serviceContextIE struct {
	db.DbIf

	*logger.NsfLogger
}

// TODO: You may need to add some fields
type serviceContext struct {
	db.DbIf

	*logger.NsfLogger
}

// TODO: Finish the initialization of the serviceContext instance
func newServiceContext(serviceContextIE *serviceContextIE) *serviceContext {

	return &serviceContext{
		DbIf: serviceContextIE.DbIf,

		NsfLogger: serviceContextIE.NsfLogger,
	}
}

// TODO: Finish the release of the serviceContext instance
func (s *serviceContext) release() {
	s.NsfLogger.SvcLog.Infoln("Release serviceContext...")

	s.NsfLogger.SvcLog.Infoln("serviceContext released")
}

// TODO: Finish the implementation of the registerServices method with an error return type
func (s *serviceContext) registerServices(nfName, sbiAddr string, sbiPort int, services []string) error {
	s.NsfLogger.SvcLog.Tracef("Registering %s services", nfName)
	info := serviceInfo{
		NfName:     nfName,
		SbiAddress: sbiAddr,
		SbiPort:    sbiPort,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	for _, service := range services {
		key := nfName + ":" + service

		if err := s.Save(key, string(data)); err != nil {
			return err
		}
	}

	return nil
}

// TODO: Finish the implementation of the deregisterServices method with an error return type
func (s *serviceContext) deregisterServices(nfName string, services []string) error {
	s.NsfLogger.SvcLog.Tracef("De-registering %s services", nfName)
	for _, service := range services {
		key := nfName + ":" + service

		if err := s.Remove(key); err != nil {
			return err
		}
	}

	return nil
}

// TODO: Finish the implementation of the discoverService method with these return values:
// (string, string, int, error) representing the NF name, SBI address, SBI port, and an error if any
func (s *serviceContext) discoverService(service string) (string, string, int, error) {
	s.NsfLogger.SvcLog.Tracef("Discovering service %s", service)
	// 取得資料庫中所有資料
	records, err := s.LoadAll()
	if err != nil {
		return "", "", 0, err
	}

	// 找出符合的 Service
	for key, value := range records {
		if !strings.HasSuffix(key, ":"+service) {
			continue
		}

		var info serviceInfo

		// JSON 轉回 struct
		if err := json.Unmarshal([]byte(value), &info); err != nil {
			return "", "", 0, err
		}

		return info.NfName, info.SbiAddress, info.SbiPort, nil
	}

	// 找不到 Service
	return "", "", 0, fmt.Errorf("service %s not found", service)
}
