// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"log"
	"strconv"
	"strings"
)

func (c *HAProxyController) updateHAProxy() error {
	needsReload := false

	err := c.apiStartTransaction()
	if err != nil {
		log.Println(err)
		return err
	}
	defer func() {
		c.apiDisposeTransaction()
	}()
	c.handleDefaultTimeouts()

	maxconnAnn, err := GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations)
	if err == nil {
		if maxconnAnn.Status == DELETED {
			err = c.handleMaxconn(nil, FrontendHTTP, FrontendHTTPS)
			if err != nil {
				return err
			}
		} else if maxconnAnn.Status != "" {
			var value int64
			value, err = strconv.ParseInt(maxconnAnn.Value, 10, 64)
			if err == nil {
				err = c.handleMaxconn(&value, FrontendHTTP, FrontendHTTPS)
				if err != nil {
					return err
				}
			}
		}
	}

	reload, err := c.handleGlobalAnnotations()
	LogErr(err)
	needsReload = needsReload || reload

	usedCerts := map[string]struct{}{}
	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		for _, ingress := range namespace.Ingresses {
			annClass, _ := GetValueFromAnnotations("ingress.class", ingress.Annotations) // default is ""
			if annClass.Value != "" && annClass.Value != c.osArgs.IngressClass {
				continue
			}
			if c.cfg.PublishService != nil && ingress.Status != DELETED {
				LogErr(c.k8s.UpdateIngressStatus(ingress, c.cfg.PublishService))
			}
			// handle Ingress rules
			for _, rule := range ingress.Rules {
				for _, path := range rule.Paths {
					reload, err = c.handlePath(namespace, ingress, rule, path)
					needsReload = needsReload || reload
					LogErr(err)
				}
			}
			//handle certs
			ingressSecrets := map[string]struct{}{}
			for _, tls := range ingress.TLS {
				if _, ok := ingressSecrets[tls.SecretName.Value]; !ok {
					ingressSecrets[tls.SecretName.Value] = struct{}{}
					reload = c.handleTLSSecret(*ingress, *tls, usedCerts)
					needsReload = needsReload || reload
				}
			}
		}
	}

	reload = c.handleDefaultCertificate(usedCerts)
	needsReload = needsReload || reload

	reload = c.handleHTTPS(usedCerts)
	needsReload = needsReload || reload

	reload, err = c.handleRateLimiting(c.cfg.HTTPS)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload

	reload, err = c.handleHTTPRedirect(c.cfg.HTTPS)
	if err != nil {
		return err
	}
	needsReload = needsReload || reload

	// reload, err = c.handleDefaultService()
	// LogErr(err)
	// needsReload = needsReload || reload

	reload, err = c.requestsTCPRefresh()
	LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.RequestsHTTPRefresh()
	LogErr(err)
	needsReload = needsReload || reload

	reload, err = c.handleTCPServices()
	LogErr(err)
	needsReload = needsReload || reload

	reload = c.refreshBackendSwitching()
	needsReload = needsReload || reload

	err = c.apiCommitTransaction()
	if err != nil {
		log.Println(err)
		return err
	}
	c.cfg.Clean()
	if needsReload {
		if err := c.HAProxyReload(); err != nil {
			log.Println(err)
		} else {
			log.Println("HAProxy reloaded")
		}
	}
	return nil
}

func (c *HAProxyController) handleMaxconn(maxconn *int64, frontends ...string) error {
	for _, frontendName := range frontends {
		if frontend, err := c.frontendGet(frontendName); err == nil {
			frontend.Maxconn = maxconn
			err1 := c.frontendEdit(frontend)
			LogErr(err1)
		} else {
			return err
		}
	}
	return nil
}

func (c *HAProxyController) handleDefaultService() (needsReload bool, err error) {
	needsReload = false
	dsvcData, _ := GetValueFromAnnotations("default-backend-service")
	dsvc := strings.Split(dsvcData.Value, "/")

	if len(dsvc) != 2 {
		return needsReload, errors.New("default service invalid data")
	}
	namespace, ok := c.cfg.Namespace[dsvc[0]]
	if !ok {
		return needsReload, errors.New("default service invalid namespace " + dsvc[0])
	}
	service, ok := namespace.Services[dsvc[1]]
	if !ok {
		return needsReload, errors.New("service " + dsvc[1] + "does not exists ")
	}
	ingress := &Ingress{
		Namespace:   namespace.Name,
		Annotations: MapStringW{},
		Rules:       map[string]*IngressRule{},
	}
	path := &IngressPath{
		ServiceName:      service.Name,
		ServicePortInt:   service.Ports[0].ServicePort,
		IsDefaultBackend: true,
	}
	return c.handlePath(namespace, ingress, &IngressRule{}, path)
}
