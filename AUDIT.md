# VasmaX 全面审计报告

## 所有问题均已修复 ✓

### 1. ✅ [i18n] `menu.choose` → `menu.select` (`internal/menu/main.go`)
### 2. ✅ [config/validator] 添加 `anytls` 到 validNodeTypes (`internal/config/validator.go`)
### 3. ✅ [subscription/salt] SubscribeURL 参数顺序修正 (`internal/subscription/salt.go`)
### 4. ✅ [subscription/manager] showLinks URL 路径统一（同 #3）
### 5. ✅ [core/manager] backupFile 权限 0700→0600 (`internal/core/manager.go`)
### 6. ✅ [api/push+sync/loop] ctx 传递到所有 API 调用 (`internal/api/push.go`, `internal/sync/loop.go`, `cmd/vasmax/main.go`)
### 7. ✅ [firewall/iptables] 删除冗余 contains 函数，ufw.go 改用 strings.Contains (`internal/firewall/iptables.go`, `internal/firewall/ufw.go`)
### 8. ✅ [nginx/template] listen 443 ssl + http2 on (`internal/nginx/template.go`)
### 9. ✅ [subscription/remote] 无 fragment 的 URI 追加 #alias (`internal/subscription/remote.go`)
### 10. ✅ [sysinfo/detect] detectPublicIP 空 IP 检查 (`internal/sysinfo/detect.go`)
### 11. ✅ [rollback/manager] 版本截断先 TrimSpace 再截取，避免 UTF-8 截断 (`internal/rollback/manager.go`)
### 12. ✅ [config/validator] TLS 版本范围校验 (`internal/config/validator.go`)
### 13. ✅ [menu/tls] TLS 版本比较改用数值 map (`internal/menu/tls.go`)
### 14. ✅ [menu/port_menu] closePort 直接使用 m.config.ExtraPorts 避免 alias 混淆 (`internal/menu/port_menu.go`)
### 15. ✅ [menu/display] 空输入重试而非退出 (`internal/menu/display.go`)
### 16. ✅ 同 #3
