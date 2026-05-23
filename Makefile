.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-ci test-frontend-smoke secret-scan

FRONTEND_CI_VITEST := \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/api/__tests__/payment.spec.ts \
	src/api/__tests__/admin.payment.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/router/__tests__/channel-monitor-routes.spec.ts \
	src/utils/__tests__/channelMonitorFeatureFlags.spec.ts \
	src/api/__tests__/settings.kiroRuntime.spec.ts \
	src/composables/__tests__/useKiroOAuth.spec.ts \
	src/components/account/__tests__/KiroAuthorizationFlow.spec.ts \
	src/views/user/__tests__/TicketsView.spec.ts \
	src/views/user/__tests__/TicketCreateView.spec.ts \
	src/views/admin/__tests__/TicketsView.spec.ts \
	src/views/admin/__tests__/TicketDetailView.spec.ts \
	src/components/tickets/__tests__/TicketEditorCard.spec.ts \
	src/components/tickets/__tests__/TicketConversationPane.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-ci
	@pnpm --dir frontend run build

test-frontend-ci:
	@pnpm --dir frontend run test:run

test-frontend-smoke:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CI_VITEST)

secret-scan:
	@python3 tools/secret_scan.py
