// Copyright 2017 Burak Serdar

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

var (
	adminPort = flag.String("adm", "8001", "Admin port (8001)")
	mockPort  = flag.String("port", "8000", "Port (8000)")
)

type (
	// AdminHandler manages mocked routes
	AdminHandler struct {
		Routes []*RouteRequest
		M      *MockHandler
	}

	// MockHandler mocks routes in adminHandler
	MockHandler struct {
		sync.RWMutex
		Router *mux.Router
	}

	// Pair is key-value pair, keys may be repeated so can't use map
	Pair struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	// Pairs is an array of pairs
	Pairs []Pair

	// ReturnData specifies what to return
	ReturnData struct {
		Status  int    `json:"status"`
		Headers Pairs  `json:"headers"`
		Body    string `json:"body"`
	}

	// RouteRequest specifies a route and what to return
	RouteRequest struct {
		Headers Pairs      `json:"headers"`
		Method  string     `json:"method"`
		Path    string     `json:"path"`
		Queries Pairs      `json:"queries"`
		Return  ReturnData `json:"return"`
	}
)

// ToMap adds pairs to the dest map
func (p Pairs) ToMap(dest map[string][]string) {
	if p != nil {
		for _, x := range p {
			if v, ok := dest[x.Key]; ok {
				dest[x.Key] = append(v, x.Value)
			} else {
				dest[x.Key] = []string{x.Value}
			}
		}
	}
}

// ToA converts pairs to a string array of pairs
func (p Pairs) ToA() []string {
	if p == nil || len(p) == 0 {
		return nil
	}
	ret := make([]string, 2*len(p))
	for i, x := range p {
		ret[2*i] = x.Key
		ret[2*i+1] = x.Value
	}
	return ret
}

// BuildRoute builds a route from the request
func (r RouteRequest) BuildRoute(router *mux.Router) (*mux.Route, error) {
	if router == nil {
		router = mux.NewRouter()
	}
	if len(r.Path) == 0 {
		return nil, errors.New("path required")
	}
	route := router.Path(r.Path)
	if len(r.Method) > 0 {
		route = route.Methods(r.Method)
	}
	pairs := r.Headers.ToA()
	if pairs != nil {
		route = route.HeadersRegexp(pairs...)
	}
	queries := r.Queries.ToA()
	if queries != nil {
		route = route.Queries(queries...)
	}
	return route, nil
}

// PairsEq returns true if pairs are set-equivalent
func PairsEq(v1, v2 Pairs) bool {
	if v1 == nil && v2 == nil {
		return true
	}
	if v1 != nil && v2 != nil {
		if len(v1) == len(v2) {
			for _, p1 := range v1 {
				found := false
				for _, p2 := range v2 {
					if p1 == p2 {
						found = true
						break
					}
				}
				if !found {
					break
				}
			}
			return true
		}
	}
	return false
}

// RoutesEq returns true if two request would yield the same path
func RoutesEq(r1, r2 *RouteRequest) bool {
	return r1.Method == r2.Method &&
		r1.Path == r2.Path &&
		PairsEq(r1.Headers, r2.Headers) &&
		PairsEq(r1.Queries, r2.Queries)
}

// AddRoute adds a new route. It may replace an equivalent route
func (h *AdminHandler) AddRoute(req RouteRequest) {
	found := false
	for _, r := range h.Routes {
		if RoutesEq(&req, r) {
			found = true
			break
		}
	}
	if !found {
		h.Routes = append(h.Routes, &req)
	}
}

// MockReqHandler returns the required response
type MockReqHandler struct {
	R RouteRequest
}

func (h MockReqHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	h.R.Return.Headers.ToMap(writer.Header())
	writer.WriteHeader(h.R.Return.Status)
	writer.Write([]byte(h.R.Return.Body))
}

// BuildRouter builds a router from all requests
func (h *AdminHandler) BuildRouter() *mux.Router {
	router := mux.NewRouter()
	for _, r := range h.Routes {
		route, _ := r.BuildRoute(router)
		route.Handler(MockReqHandler{R: *r})
	}
	return router
}

// ProcessStream processes the given stream, parses it and creates routes
func (h *AdminHandler) ProcessStream(rd io.Reader) ([]RouteRequest, error) {
	var reqs []RouteRequest
	data, err := ioutil.ReadAll(rd)
	if err == nil {
		err = json.Unmarshal(data, &reqs)
		if err != nil {
			reqs = make([]RouteRequest, 1)
			err = json.Unmarshal(data, &reqs[0])
		}
		if err == nil {
			h.M.Lock()
			defer h.M.Unlock()

			for _, req := range reqs {
				_, err := req.BuildRoute(nil)
				if err == nil {
					h.AddRoute(req)
				} else {
					break
				}
			}
			if err == nil {
				h.M.Router = h.BuildRouter()
			}
		}
	}
	return reqs, err
}

func (h *AdminHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		reqs, err := h.ProcessStream(request.Body)
		if err == nil {
			writer.WriteHeader(http.StatusOK)
			ret, _ := json.Marshal(reqs)
			writer.Write(ret)
		} else {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(err.Error()))
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *MockHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	h.RLock()
	if h.Router == nil {
		writer.WriteHeader(http.StatusNotFound)
	} else {
		h.Router.ServeHTTP(writer, request)
	}
	h.RUnlock()
}

func main() {
	flag.Parse()

	m := MockHandler{}
	a := AdminHandler{Routes: make([]*RouteRequest, 0), M: &m}

	for _, f := range flag.Args() {
		file, err := os.Open(f)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		_, err = a.ProcessStream(file)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		file.Close()
	}

	admSrv := &http.Server{
		Handler:      &a,
		Addr:         ":" + *adminPort,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	go func() {
		admSrv.ListenAndServe()
	}()

	mockSrv := &http.Server{
		Handler:      &m,
		Addr:         ":" + *mockPort,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	fmt.Printf("%v\n", mockSrv.ListenAndServe())
}
