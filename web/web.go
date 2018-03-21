package web

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"pinger/logger"
	"pinger/pools"
)

/*
Params - keep default wariables. Old stuff, must be reworked or deleted
 */
type Params struct {
	DefaultProbes   int
	DefaultInterval int64
	DefaultURL      string
}

/*
NewWeb - returns new Params with parameters
 */
func NewWeb(p int, i int64, u string) *Params {
	w := Params{
		DefaultProbes:   p,
		DefaultInterval: i,
		DefaultURL:      u,
	}

	return &w
}

// Store - same as 'GetOrStore', but without removing non-existing hosts
func (ws *Params) Store(w http.ResponseWriter, r *http.Request) {
	ws.getOrStore(w, r, false)
}



// GetOrStore accepts json array with parameters
func (ws *Params) GetOrStore(w http.ResponseWriter, r *http.Request) {
	ws.getOrStore(w, r, true)
}

//func (ws *Params) getOrStore(w http.ResponseWriter, r *http.Request, removeOld bool) {
func (ws *Params) getOrStore(w io.Writer, r *http.Request, removeOld bool) {

	// todo: remove layer [topics:[]] from json
	jsonParams := make(map[string]interface{})

	body, bodyErr := ioutil.ReadAll(r.Body)
	if bodyErr != nil {
		ReturnError(w, r, fmt.Sprintf("Error getting input: %s", bodyErr.Error()), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	jsonErr := json.Unmarshal(body, &jsonParams)
	if nil != jsonErr {
		ReturnError(w, r, fmt.Sprintf("Cannot parse json body: %s", jsonErr.Error()), http.StatusBadRequest)
		return
	}

	//topics, err := ws.getTopics(jsonParams)
	topics, err := pools.ParseTopics(jsonParams, ws.DefaultProbes, ws.DefaultInterval)
	if err != nil {
		ReturnError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	//logger.Debug("TOPICS: %#v", topics)

	//ret := pools.GlobalPool.GetOrStore(topics)
	result := pools.TopicPool.GetOrStore(topics, removeOld)
	// return json report with current objects
	bytes, e := json.Marshal(result)
	if e != nil {
		ReturnError(w, r, fmt.Sprintf("Cannot marshal result: %s", e.Error()), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "%s", string(bytes))
}

/*
ReturnError returns an http error and logs it into error log
*/
//func ReturnError(w http.ResponseWriter, r *http.Request, err string, status int) {
func ReturnError(w io.Writer, r *http.Request, err string, status int) {
	//http.Error(w, err, status)
	fmt.Fprintf(w, fmt.Sprintf(`{"ok":false, "message":"%s"}`, err))
	logger.Err("[web]: Error: %s (%s)", err, r.URL)
}
