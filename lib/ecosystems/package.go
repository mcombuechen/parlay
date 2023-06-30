/*
 * © 2023 Snyk Limited All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ecosystems

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/package-url/packageurl-go"
	"github.com/remeh/sizedwaitgroup"

	"github.com/snyk/parlay/ecosystems/packages"
)

const server = "https://packages.ecosyste.ms/api/v1"

func GetPackageData(purl packageurl.PackageURL) (*packages.GetRegistryPackageResponse, error) {
	client, err := packages.NewClientWithResponses(server)
	if err != nil {
		return nil, err
	}

	// Ecosyste.ms has a purl based API, but unfortunately slower
	// so we break the purl down to registry and name values locally
	// params := packages.LookupPackageParams{Purl: &p}
	// resp, err := client.LookupPackageWithResponse(context.Background(), &params)
	name := purlToEcosystemsName(purl)
	registry := purlToEcosystemsRegistry(purl)
	resp, err := client.GetRegistryPackageWithResponse(context.Background(), registry, name)

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func GetPackageDataConcurrently(purlsByID map[string]string, concurrency int) map[string]*packages.Package {
	mtx := sync.Mutex{}
	wg := sizedwaitgroup.New(concurrency)
	results := make(map[string]*packages.Package)

	for id, p := range purlsByID {
		wg.Add()

		go func(p, id string) {
			defer wg.Done()
			defer mtx.Unlock()

			purl, err := packageurl.FromString(p)
			if err != nil {
				return
			}

			resp, err := GetPackageData(purl)
			if err != nil {
				return
			}

			packageData := resp.JSON200
			if packageData != nil {
				mtx.Lock()
				results[id] = packageData
			}
		}(p, id)
	}

	wg.Wait()

	return results
}

func purlToEcosystemsRegistry(purl packageurl.PackageURL) string {
	return map[string]string{
		"npm":       "npmjs.org",
		"golang":    "proxy.golang.org",
		"nuget":     "nuget.org",
		"hex":       "hex.pm",
		"maven":     "repo1.maven.org",
		"pypi":      "pypi.org",
		"composer":  "packagist.org",
		"gem":       "rubygems.org",
		"cargo":     "crates.io",
		"cocoapods": "cocoapod.org",
		"apk":       "alpine",
	}[purl.Type]
}

func purlToEcosystemsName(purl packageurl.PackageURL) string {
	var name string
	// npm names in the ecosyste.ms API include the purl namespace
	// followed by a / and are url encoded. Other package managers
	// appear to separate the purl namespace and name with a :
	if purl.Type == "npm" {
		if purl.Namespace != "" {
			name = url.QueryEscape(fmt.Sprintf("%s/%s", purl.Namespace, purl.Name))
		} else {
			name = purl.Name
		}
	} else {
		if purl.Namespace != "" {
			name = fmt.Sprintf("%s:%s", purl.Namespace, purl.Name)
		} else {
			name = purl.Name
		}
	}
	return name
}
