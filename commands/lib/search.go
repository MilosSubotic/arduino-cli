// This file is part of arduino-cli.
//
// Copyright 2020 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package lib

import (
	"context"
	"errors"
	"strings"

	"github.com/arduino/arduino-cli/arduino/libraries/librariesindex"
	"github.com/arduino/arduino-cli/arduino/libraries/librariesmanager"
	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/commands"
	"github.com/lithammer/fuzzysearch/fuzzy"
	semver "go.bug.st/relaxed-semver"
)

// LibrarySearch FIXMEDOC
func LibrarySearch(ctx context.Context, req *rpc.LibrarySearchReq) (*rpc.LibrarySearchResp, error) {
	lm := commands.GetLibraryManager(req.GetInstance().GetId())
	if lm == nil {
		return nil, errors.New("invalid instance")
	}

	return searchLibrary(req, lm)
}

func searchLibrary(req *rpc.LibrarySearchReq, lm *librariesmanager.LibrariesManager) (*rpc.LibrarySearchResp, error) {
	query := req.GetQuery()
	res := []*rpc.SearchedLibrary{}
	status := rpc.LibrarySearchStatus_success

	// If the query is empty all libraries are returned
	if strings.Trim(query, " ") == "" {
		for _, lib := range lm.Index.Libraries {
			res = append(res, indexLibraryToRPCSearchLibrary(lib))
		}
		return &rpc.LibrarySearchResp{Libraries: res, Status: status}, nil
	}

	// maximumSearchDistance is the maximum Levenshtein distance accepted when using fuzzy search.
	// This value is completely arbitrary and picked randomly.
	maximumSearchDistance := 150
	// Use a lower distance for shorter query or the user might be flooded with unrelated results
	if len(query) <= 4 {
		maximumSearchDistance = 40
	}

	// Removes some chars from query strings to enhance results
	cleanQuery := strings.Map(func(r rune) rune {
		switch r {
		case '_':
		case '-':
		case ' ':
			return -1
		}
		return r
	}, query)
	for _, lib := range lm.Index.Libraries {
		// Use both uncleaned and cleaned query
		for _, q := range []string{query, cleanQuery} {
			toTest := []string{lib.Name, lib.Latest.Paragraph, lib.Latest.Sentence}
			for _, rank := range fuzzy.RankFindNormalizedFold(q, toTest) {
				if rank.Distance < maximumSearchDistance {
					res = append(res, indexLibraryToRPCSearchLibrary(lib))
					goto nextLib
				}
			}
		}
	nextLib:
	}

	return &rpc.LibrarySearchResp{Libraries: res, Status: status}, nil
}

// indexLibraryToRPCSearchLibrary converts a librariindex.Library to rpc.SearchLibrary
func indexLibraryToRPCSearchLibrary(lib *librariesindex.Library) *rpc.SearchedLibrary {
	releases := map[string]*rpc.LibraryRelease{}
	for str, rel := range lib.Releases {
		releases[str] = getLibraryParameters(rel)
	}
	latest := getLibraryParameters(lib.Latest)

	return &rpc.SearchedLibrary{
		Name:     lib.Name,
		Releases: releases,
		Latest:   latest,
	}
}

// getLibraryParameters FIXMEDOC
func getLibraryParameters(rel *librariesindex.Release) *rpc.LibraryRelease {
	return &rpc.LibraryRelease{
		Author:           rel.Author,
		Version:          rel.Version.String(),
		Maintainer:       rel.Maintainer,
		Sentence:         rel.Sentence,
		Paragraph:        rel.Paragraph,
		Website:          rel.Website,
		Category:         rel.Category,
		Architectures:    rel.Architectures,
		Types:            rel.Types,
		License:          rel.License,
		ProvidesIncludes: rel.ProvidesIncludes,
		Dependencies:     getLibraryDependenciesParameter(rel.GetDependencies()),
		Resources: &rpc.DownloadResource{
			Url:             rel.Resource.URL,
			Archivefilename: rel.Resource.ArchiveFileName,
			Checksum:        rel.Resource.Checksum,
			Size:            rel.Resource.Size,
			Cachepath:       rel.Resource.CachePath,
		},
	}
}

func getLibraryDependenciesParameter(deps []semver.Dependency) []*rpc.LibraryDependency {
	res := []*rpc.LibraryDependency{}
	for _, dep := range deps {
		res = append(res, &rpc.LibraryDependency{
			Name:              dep.GetName(),
			VersionConstraint: dep.GetConstraint().String(),
		})
	}
	return res
}
