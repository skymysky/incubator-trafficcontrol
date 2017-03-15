/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package tmcheck

import (
	"encoding/json"
	"fmt"
	"github.com/apache/incubator-trafficcontrol/traffic_monitor_golang/traffic_monitor/enum"
	to "github.com/apache/incubator-trafficcontrol/traffic_ops/client"
	"io/ioutil"
	"time"
)

const PeerPollMax = time.Duration(10) * time.Second

const TrafficMonitorStatsPath = "/publish/Stats"

// TrafficMonitorStatsJSON represents the JSON returned by Traffic Monitor's Stats endpoint. This currently only contains the Oldest Polled Peer Time member, as needed by this library.
type TrafficMonitorStatsJSON struct {
	Stats TrafficMonitorStats `json:"stats"`
}

// TrafficMonitorStats represents the internal JSON object returned by Traffic Monitor's Stats endpoint. This currently only contains the Oldest Polled Peer Time member, as needed by this library.
type TrafficMonitorStats struct {
	OldestPolledPeerTime int `json:"Oldest Polled Peer Time (ms)"`
}

func GetOldestPolledPeerTime(uri string) (time.Duration, error) {
	resp, err := getClient().Get(uri + TrafficMonitorStatsPath)
	if err != nil {
		return time.Duration(0), fmt.Errorf("reading reply from %v: %v\n", uri, err)
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return time.Duration(0), fmt.Errorf("reading reply from %v: %v\n", uri, err)
	}

	stats := TrafficMonitorStatsJSON{}
	if err := json.Unmarshal(respBytes, &stats); err != nil {
		return time.Duration(0), fmt.Errorf("unmarshalling: %v", err)
	}

	oldestPolledPeerTime := time.Duration(stats.Stats.OldestPolledPeerTime) * time.Millisecond

	return oldestPolledPeerTime, nil
}

func ValidatePeerPoller(uri string) error {
	lastPollTime, err := GetOldestPolledPeerTime(uri)
	if err != nil {
		return fmt.Errorf("failed to get oldest peer time: %v", err)
	}
	if lastPollTime > PeerPollMax {
		return fmt.Errorf("Peer poller is dead, last poll was %v ago", lastPollTime)
	}
	return nil
}

func ValidateAllPeerPollers(toClient *to.Session, includeOffline bool) (map[enum.TrafficMonitorName]error, error) {
	servers, err := GetMonitors(toClient, includeOffline)
	if err != nil {
		return nil, err
	}
	errs := map[enum.TrafficMonitorName]error{}
	for _, server := range servers {
		uri := fmt.Sprintf("http://%s.%s", server.HostName, server.DomainName)
		errs[enum.TrafficMonitorName(server.HostName)] = ValidatePeerPoller(uri)
	}
	return errs, nil
}

func PeerPollersValidator(
	tmURI string,
	toClient *to.Session,
	interval time.Duration,
	grace time.Duration,
	onErr func(error),
	onResumeSuccess func(),
	onCheck func(error),
) {
	wrapValidatePeerPoller := func(uri string, _ *to.Session) error { return ValidatePeerPoller(uri) }
	Validator(tmURI, toClient, interval, grace, onErr, onResumeSuccess, onCheck, wrapValidatePeerPoller)
}

func PeerPollersAllValidator(
	toClient *to.Session,
	interval time.Duration,
	includeOffline bool,
	grace time.Duration,
	onErr func(enum.TrafficMonitorName, error),
	onResumeSuccess func(enum.TrafficMonitorName),
	onCheck func(enum.TrafficMonitorName, error),
) {
	AllValidator(toClient, interval, includeOffline, grace, onErr, onResumeSuccess, onCheck, ValidateAllPeerPollers)
}