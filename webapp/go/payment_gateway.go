package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var erroredUpstream = errors.New("errored upstream")

type paymentGatewayPostPaymentRequest struct {
	Amount int `json:"amount"`
}

type paymentGatewayGetPaymentsResponseOne struct {
	Amount int    `json:"amount"`
	Status string `json:"status"`
}

func (h *apiHandler) requestPaymentGatewayPostPayment(ctx context.Context, token string, param *paymentGatewayPostPaymentRequest, retrieveRidesCount func() (int, error)) error {
	b, err := json.Marshal(param)
	if err != nil {
		return err
	}

	// 失敗したらとりあえずリトライ
	// FIXME: 社内決済マイクロサービスのインフラに異常が発生していて、同時にたくさんリクエストすると変なことになる可能性あり
	retry := 0
	paymentsURL := h.paymentGatewayURL + "/payments"
	authorization := "Bearer " + token
	for {
		err := func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, paymentsURL, bytes.NewBuffer(b))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", authorization)

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			if res.StatusCode == http.StatusNoContent {
				return nil
			}
			// エラーが返ってきても成功している場合があるので、社内決済マイクロサービスに問い合わせ
			getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, paymentsURL, bytes.NewBuffer([]byte{}))
			if err != nil {
				return err
			}
			getReq.Header.Set("Authorization", authorization)

			getRes, err := http.DefaultClient.Do(getReq)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			// GET /payments は障害と関係なく200が返るので、200以外は回復不能なエラーとする
			if getRes.StatusCode != http.StatusOK {
				return fmt.Errorf("[GET /payments] unexpected status code (%d)", getRes.StatusCode)
			}
			var payments []paymentGatewayGetPaymentsResponseOne
			if err := json.NewDecoder(getRes.Body).Decode(&payments); err != nil {
				return err
			}

			rides, err := retrieveRidesCount()
			if err != nil {
				return err
			}

			if rides != len(payments) {
				return fmt.Errorf("unexpected number of payments: %d != %d. %w", rides, len(payments), erroredUpstream)
			}

			return nil
		}()
		if err == nil {
			return nil
		}
		if retry < 5 {
			retry++
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			return err
		}
	}
}
