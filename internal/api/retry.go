package api

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// 重试间隔：2s, 4s, 8s（指数退避）
var retryDelays = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
}

// doWithRetry 带指数退避重试的请求执行
// 仅对 5xx 和网络错误重试，4xx（非 304）不重试
// 记录错误日志含状态码和响应体摘要
func doWithRetry(fn func() (*http.Response, error), logger *logrus.Logger) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		resp, err := fn()
		if err != nil {
			// 网络错误，重试
			lastErr = err
			if attempt < len(retryDelays) {
				logger.WithFields(logrus.Fields{
					"attempt": attempt + 1,
					"delay":   retryDelays[attempt],
					"error":   err.Error(),
				}).Warn("请求网络错误，准备重试")
				time.Sleep(retryDelays[attempt])
				continue
			}
			return nil, fmt.Errorf("请求失败（已重试 %d 次）: %w", len(retryDelays), lastErr)
		}

		// 4xx 不重试（非 304）
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			logger.WithField("status", resp.StatusCode).Error("API 返回客户端错误，不重试")
			return resp, nil
		}

		// 5xx 重试
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("服务端错误: %d, body: %s", resp.StatusCode, truncateBody(body))
			if attempt < len(retryDelays) {
				logger.WithFields(logrus.Fields{
					"status":  resp.StatusCode,
					"attempt": attempt + 1,
					"delay":   retryDelays[attempt],
					"body":    truncateBody(body),
				}).Warn("API 返回服务端错误，准备重试")
				time.Sleep(retryDelays[attempt])
				continue
			}
			return nil, fmt.Errorf("请求失败（已重试 %d 次）: %w", len(retryDelays), lastErr)
		}

		// 2xx/3xx 成功
		return resp, nil
	}

	return nil, fmt.Errorf("请求失败（已重试 %d 次）: %w", len(retryDelays), lastErr)
}
