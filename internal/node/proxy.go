package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/threefoldtech/go-rmb"
)

const (
	errThreshold = 4 // return error after failed 4 polls
)

type ProxyBus struct {
	endpoint string
	twinID   uint32
}

func NewProxyBus(endpoint string, twinID uint32) *ProxyBus {
	if len(endpoint) != 0 && endpoint[len(endpoint)-1] == '/' {
		endpoint = endpoint[:len(endpoint)-1]
	}
	return &ProxyBus{
		endpoint,
		twinID,
	}
}

func (r *ProxyBus) requestEndpoint(twinid uint32) string {
	return fmt.Sprintf("%s/twin/%d", r.endpoint, twinid)
}

func (r *ProxyBus) resultEndpoint(twinid uint32, retqueue string) string {
	return fmt.Sprintf("%s/twin/%d/%s", r.endpoint, twinid, retqueue)
}

func (r *ProxyBus) Call(ctx context.Context, twin uint32, fn string, data interface{}, result interface{}) error {
	bs, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "failed to serialize request data")
	}

	msg := rmb.Message{
		Version:    1,
		Expiration: 3600,
		Command:    fn,
		TwinSrc:    int(r.twinID),
		TwinDst:    []int{int(twin)},
		Data:       base64.StdEncoding.EncodeToString(bs),
	}
	bs, err = json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "failed to serialize message")
	}
	resp, err := http.Post(r.requestEndpoint(twin), "application/json", bytes.NewBuffer(bs))
	if err != nil {
		return errors.Wrap(err, "error sending request")
	}
	if resp.StatusCode != http.StatusOK {
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("error parsing response body: %s", err.Error())
		}
		return fmt.Errorf("non ok return code: %d, body: %s", resp.StatusCode, string(bs))
	}
	var res ProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return errors.Wrap(err, "failed to decode proxy response body")
	}
	msg, err = r.pollResponse(ctx, twin, res.Retqueue)
	if err != nil {
		return errors.Wrap(err, "couldn't poll response")
	}
	// errorred ?
	if len(msg.Err) != 0 {
		return errors.New(msg.Err)
	}

	// not expecting a result
	if result == nil {
		return nil
	}

	if len(msg.Data) == 0 {
		return fmt.Errorf("no response body was returned")
	}
	if err := json.Unmarshal([]byte(msg.Data), result); err != nil {
		return errors.Wrap(err, "failed to decode response body")
	}

	return nil
}

func (r *ProxyBus) pollResponse(ctx context.Context, twin uint32, retqueue string) (rmb.Message, error) {
	ts := time.NewTicker(1 * time.Second)
	errCount := 0
	var err error
	for {
		select {
		case <-ts.C:
			if errCount == errThreshold {
				return rmb.Message{}, err
			}
			resp, lerr := http.Get(r.resultEndpoint(twin, retqueue))
			if lerr != nil {
				log.Printf("failed to send result-fetching request: %s", err.Error())
				errCount += 1
				err = lerr
				continue
			}
			if resp.StatusCode == 404 {
				// message not there yet
				continue
			}
			if resp.StatusCode != http.StatusOK {
				bs, e := io.ReadAll(resp.Body)
				if e != nil {
					log.Printf("error parsing response body: %s", e.Error())
				}
				log.Printf("non ok status code: %d, body: %s", resp.StatusCode, bs)
				err = fmt.Errorf("non ok return code: %d, body: %s", resp.StatusCode, bs)
				errCount += 1
				continue
			}
			var msgs []rmb.Message
			if lerr := json.NewDecoder(resp.Body).Decode(&msgs); lerr != nil {
				err = lerr
				errCount += 1
				continue
			}
			if len(msgs) == 0 {
				// nothing there yet
				continue
			}
			return msgs[0], nil
		case <-ctx.Done():
			return rmb.Message{}, errors.New("context cancelled")
		}
	}
}

type ProxyResponse struct {
	Retqueue string
}