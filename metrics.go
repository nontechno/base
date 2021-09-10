// Copyright 2021 The NonTechno Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type Metric interface {
	Post(value string)
	Update(interface{})
}

type Counter interface {
	Add(int64)
	Set(int64)
}

type Index = uint16

func CreateNewMetric(id, name, units string) Metric {
	return Metric(createNewMetric(id, name, units))
}

func CreateNewCounter(id, name, units string) Counter {
	return Counter(createNewMetric(id, name, units))
}

func createNewMetric(id, name, units string) *metricClient {
	initPipes()

	metricsGuard.Lock()
	if !metricsInitialized {
		metricsInitialized = true
		go clientCollector()
		go clientSender()
	}
	metricsGuard.Unlock()

	sender := metricClient{
		id:        id,
		name:      name,
		units:     units,
		index:     Index(atomic.AddUint32(&vacantMetricSlot, 1)),
		published: false,
	}
	sender.register()

	return &sender
}

func OneTimeMetric(name string, value interface{}) {
	CreateNewMetric("one.time.metric:"+name, name, "").Update(value)
}

type metricClient struct {
	id    string
	name  string
	units string

	published bool
	index     Index
	value     string
	last      time.Time

	counter int64
}

type clentUpdate struct {
	index Index
	value string
}

func (cu *clentUpdate) write(media io.Writer) {
	writePacket(media, cu.index, cu.value)

	/*

		var index [2]byte
		index[0] = byte(cu.index >> 8)
		index[1] = byte(cu.index & 0x00ff)
		media.Write(index[:])

		data := []byte(cu.value)
		media.Write([]byte{byte(len(data) & 0x000000ff)})
		media.Write(data)
	*/
}

func (ms *metricClient) register() {
	metricsGuard.Lock()

	if metricsStore == nil {
		metricsStore = make([]*metricClient, 100)
	} else if len(metricsStore) == cap(metricsStore) { // todo: test this
		store := make([]*metricClient, len(metricsStore), cap(metricsStore)*2)
		copy(store, metricsStore)
		metricsStore = store
	}
	metricsStore[ms.index] = ms
	metricsGuard.Unlock()
}

func (ms *metricClient) Update(value interface{}) {
	if value == nil {
		ms.Post("null")
		return
	}

	switch actual := value.(type) {
	case string:
		ms.Post(actual)
	default:
		ms.Post(fmt.Sprintf("%v", value))
	}
}

func (ms *metricClient) Post(value string) {
	if ms.value != value {
		ms.value = value
		ms.last = time.Now()

		metricsPipe <- clentUpdate{
			index: ms.index,
			value: ms.value,
		}
	}
}

func (ms *metricClient) Add(delta int64) {
	value := atomic.AddInt64(&ms.counter, delta)
	ms.Post(fmt.Sprintf("%v", value))
}

func (ms *metricClient) Set(value int64) {
	atomic.StoreInt64(&ms.counter, value)
	ms.Post(fmt.Sprintf("%v", value))
}

func (ms *metricClient) publish(media io.Writer) {
	const separator = "\000"
	value := ms.id + separator + ms.name + separator + ms.units + separator + ms.value + separator + separator
	writePacket(media, ms.index|0x8000, value)
}

var (
	vacantMetricSlot uint32
	metricsGuard     sync.Mutex
	metricsStore     []*metricClient

	metricsInitialized = false
	metricsPipe        = make(chan clentUpdate, 1234)
	tobesentPipe       = make(chan []clentUpdate, 123)
)

func clientCollector() {

	updates := map[Index]string{}
	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case update := <-metricsPipe:
			updates[update.index] = update.value

		case <-ticker.C:
			// fmt.Println("Tick at", t)
			if len(updates) > 0 {
				packet := make([]clentUpdate, 0, len(updates))
				for index, value := range updates {
					packet = append(packet, clentUpdate{index: index, value: value})
				}
				updates = map[Index]string{}
				tobesentPipe <- packet
			}
		}
	}
}

func publishNamesAndUnits(media io.Writer, force bool) {
	metricsGuard.Lock()
	defer metricsGuard.Unlock()

	for _, metric := range metricsStore {
		if metric != nil && (metric.published == false || force) {
			metric.published = true
			metric.publish(media)
		}
	}
}

func sendStaticMetricInfo() error {
	if pipeMetrics != nil {
		media := bytes.Buffer{}
		publishNamesAndUnits(&media, true)

		if media.Len() > 0 {
			if _, err := media.WriteTo(pipeMetrics); err != nil {
				return err
			}
		}
	}
	return success
}

func clientSender() {

	firstRun := true
	for {
		select {
		case packet := <-tobesentPipe: // this one is sent on timer...
			if len(packet) > 0 {
				if pipeMetrics != nil {

					media := bytes.Buffer{}
					// publish names-n-units
					if firstRun {
						firstRun = false
						publishNamesAndUnits(&media, false)
					}

					// marshal
					for _, v := range packet {
						v.write(&media)
					}

					if _, err := media.WriteTo(pipeMetrics); err != nil {
						fmt.Printf("failed to write: %v\n", err)
					}

					// send
				} else {
					// drop it on the floor ...
					// an alternative would be to collect received data (in a map) and send it upon new connection ...
					_ = packet
				}
			}
		}
	}
}

func writePacket(media io.Writer, index Index, value string) {
	var buff [2]byte
	buff[0] = byte(index >> 8)
	buff[1] = byte(index & 0x00ff)
	if n, err := media.Write(buff[:]); err != nil || n != len(buff) {
		// todo: error handling
	}

	data := []byte(value)
	if len(data) > 255 {
		// todo: error handling
	}

	media.Write([]byte{byte(len(data) & 0x000000ff)})
	media.Write(data)

	// todo: error handling
}
