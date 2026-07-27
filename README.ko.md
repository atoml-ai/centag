# Centag

<p align="center">
  <strong>당신의 LLM 프록시 허브 — 파이프라인이 곧 전략</strong><br/>
  범용 대규모 모델 프록시 게이트웨이. 모든 백엔드 LLM 제공자를 통합 관리하고, API Key 풀과 맞춤 프록시 전략을 지원하며, 커스터마이즈 가능한 파이프라인과 개방형 플러그인 아키텍처로 클라이언트 Agent 동작을 정의합니다.<br/>
  <em>중계소로 쓸 수 있지만, 중계소에 그치지 않습니다.</em>
</p>

<p align="center">
  <a href="https://github.com/atoml-ai/centag/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License" /></a>
  <img src="https://img.shields.io/badge/go-1.25+-00ADD8?logo=go" alt="Go Version" />
  <a href="https://github.com/atoml-ai/centag/releases"><img src="https://img.shields.io/github/v/release/atoml-ai/centag" alt="Release" /></a>
  <a href="https://github.com/atoml-ai/centag/releases"><img src="https://img.shields.io/github/downloads/atoml-ai/centag/total" alt="Downloads" /></a>
</p>

<p align="center">
  <a href="README.md">English</a> | <a href="README.zh-CN.md">简体中文</a> | <a href="README.ja.md">日本語</a> | 한국어 | <a href="README.ru.md">Русский</a> | <a href="README.es.md">Español</a>
</p>

---

## 해결하는 문제

일반적인 LLM 「중계소」는 요청을 그대로 전달할 뿐입니다. Key가 죽으면 손으로 바꾸고, 모델이 틀리면 다시 설정하고, Agent를 늘릴 때마다 또 설정——전략은 각 도구에 흩어져 있고, 게이트웨이 자체에는 전략이 없습니다.

**Centag는 중계소가 아니라 오케스트레이션 가능한 프록시 허브입니다.** 백엔드 풀, 장애 대응·디그레이드, 시나리오 라우팅, 계량·과금을 하나의 파이프라인에 모으고, Agent 측은 거의 무감합니다.

| 능력 | 얻는 것 |
|---|---|
| **백엔드 LLM 풀 관리** | OpenAI, Anthropic, 智谱, Ollama 및 호환 엔드포인트를 통합 관리; 다중 Key·다중 백엔드를 Web 한곳에서 설정 |
| **자동 장애 대응 · 매칭 · 디그레이드** | 제한 시 Key 자동 로테이션; 장애 시 백엔드 전환; 모델 능력·부하로 최적 출구 매칭 |
| **모델 라우팅** | 질문 유형에 따라 백엔드 모델을 실시간 전환; 동일 세션·동일 작업 안에서도 지능형 동적 교체, 클라이언트 재설정 불필요 |
| **Agent 시나리오 전환** | 코딩, Q&A 등 시나리오마다 파이프라인——시나리오 변경 = 전략 변경, Agent는 무감 |
| **빠른 Agent 접속** | 자주 쓰는 Agent는 설정 원클릭 기록; `centag wrap` 프로세스 프록시로 무변경 접속; 미지원은 Web UI 설정 가이드. 지원 목록 지속 확대 |
| **System Prompt 전략** | 클라이언트 system prompt 투과·추가·치환 지원——Agent 페르소나를 유지하거나, 시나리오별 규범을 겹치거나 일괄 덮어쓰기, 파이프라인 단위로 유연 설정 |
| **계량과 과금** | 요청·백엔드·모델 단위로 Token과 비용 추적 |
| **고성능 무손실 접속** | 투명 전달과 SSE 패스스루——프로토콜 호환·저오버헤드, 업스트림 의미를 최대한 유지 |

---

## 핵심 강점

### 시각적 파이프라인 오케스트레이션

중계소는 전달만 합니다. **Centag는 요청의 전체 생명주기를 설계하게 합니다**——캔버스에서 DAG를 드래그 앤 드롭하고, 파이프라인이 곧 전략입니다.

**내장 노드 16종**을 자유롭게 조합:

| 노드 | Kind | 역할 |
|------|------|------|
| Generator | `llm.generate` | 임의 LLM 백엔드로 생성 |
| Router | `route.decide` | 의도·키워드·LLM 분류로 분기 |
| Scheduler | `scheduling.decide` | 백엔드 간 스마트 스케줄링·매칭 |
| Transparent Forward | `proxy.transparent_forward` | 원시 HTTP 프록시 (SSE 투과) |
| Aggregator | `aggregate.merge` | 병렬 생성 병합 / 투표 / 최선 선택 |
| Reviewer | `quality.review` | 상위 답변 채점·감사 |
| Memory | `memory.query` | 클라우드 메모리 / 로컬 벡터에서 문맥 회상 |
| Audit | `audit.safety` | 콘텐츠 심사·안전 필터 |
| Token Usage | `metrics.token_usage` | Token 소비·비용 추적 |
| Cache | `cache.access` | 캐시 읽기/쓰기 (정확 / 의미 / 하이브리드) |
| Processor | `content.transform` | 콘텐츠 변환·후처리 |
| Tool Call | `inject.tool_call` | Function Calling 도구 주입 |
| Prompt Ops | `prompt.ops` | 사용자 Prompt 전처리 |
| Output Post-ops | `prompt.postprocess` | 출력 후처리 |
| Loop Controller | — | 반복 워크플로용 루프 제어 |
| Plugin Node | *(원격 / 업무)* | HTTP 또는 Go SDK 커스텀 노드 |

**파이프라인 = 전략.** 시나리오 변경 → 파이프라인 변경 → Agent 코드는 한 줄도 안 바꿈.

| 시나리오 | 파이프라인 예시 |
|------|-----------|
| 코딩 어시스턴트 | 라우팅 → 코드 특화 모델 → 코드 리뷰 |
| 스마트 스케줄링 | 의도 인식 → 모델 능력 매칭 → 장애 대응 |
| 기업 컴플라이언스 | 안전 심사 → 생성 → PII 마스킹 → 감사 |
| 고객지원 / RAG | 메모리 또는 검색 회상 → 생성 → 품질 검수 |

### 통합 백엔드와 Key 풀

| 능력 | 설명 |
|------|------|
| **다중 백엔드 관리** | 주요 벤더와 OpenAI 호환 엔드포인트를 Web에서 통합 관리 |
| **API Key 풀링** | 백엔드마다 여러 Key; 제한·장애 시 자동 로테이션 |
| **자동 장애 대응·디그레이드** | Key 실패 → 다음 Key; 백엔드 장애 → 다음 백엔드 |
| **스마트 매칭** | 가중치·우선순위·모델 능력으로 최적 출구 선택 |
| **비용 추적** | 요청·백엔드·모델 단위로 Token과 비용 집계 |

### 빠른 Agent 접속 — 세 가지 방식

업무 코드를 바꾸지 않고 Agent를 Centag에 연결. 적응 수준에 따라 선택:

| 방식 | 적합 | 설명 |
|------|------|------|
| **원클릭 설정 기록** | 이미 적응된 주요 Agent | Web UI에서 Base URL / API Key 등을 한 번에 기록, 바로 사용 |
| **centag wrap 프로세스 프록시** | 설정을 전혀 바꾸고 싶지 않을 때 | 프로세스급 투명 프록시. Agent 설정·코드 없이 트래픽을 Centag로 |
| **UI 설정 가이드** | 아직 원클릭 미지원 Agent | 게이트웨이를 수동으로 가리키는 단계를 페이지에서 안내 |

주요 Agent 지원은 계속 늘어납니다. 미지원도 가이드 또는 wrap으로 먼저 연결할 수 있습니다.

```bash
# Centag 시작
centag

# wrap 예시——Agent 설정 변경 없음
centag wrap run -- opencode

# 자가 진단
centag wrap doctor
```

### 개방형 플러그인 생태계

파이프라인 노드는 확장 가능: Go SDK 로컬 플러그인, 또는 임의 언어의 원격 HTTP 플러그인.

```go
type NodePlugin interface {
    Descriptor() NodePluginDescriptor
    ValidateConfig(config NodeConfig) error
    Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}
```

원격 플러그인 규약:

```
GET  /.well-known/centag-node-plugin.json   →  자동 발견
POST /validate                               →  설정 검증
POST /execute                                →  노드 실행
```

---

## 빠른 시작

```bash
# 1. 설치 (택일)
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
# 또는
npm install -g @atomlai/centag

# 2. 시작
centag

# 3. Web UI → http://localhost:20060 → 첫 백엔드 추가

# 4. Agent 연결 (원클릭 설정, 또는 wrap 무변경)
centag wrap run -- opencode
```

완료. 트래픽은 Centag 경유: 공유 백엔드 풀, 자동 장애 대응, 모델 라우팅, 비용 가시화.

### 기타 설치 방법

<details>
<summary>npm (전역 경로 변경 없음)</summary>

```bash
npx --yes @atomlai/centag
```
</details>

<details>
<summary>오프라인 / 폐쇄망</summary>

```bash
npm install -g @atomlai/centag-offline
```
</details>

<details>
<summary>Docker (소스)</summary>

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # 필요 시 수정
./start.sh docker build personal                     # 이미지 빌드
./start.sh docker up personal                        # 컨테이너 시작
```

관리 UI: http://localhost:20060 · 중지: `./start.sh docker down`

영구화 데이터는 `deploy/docker/data/` 디렉토리에 저장 (첫 시작 시 자동 생성)。

<details>
<summary>네이티브 Docker 명령어 (대체)</summary>

```bash
# 빌드
docker build -t centag-personal:latest \
  --build-arg DIST_NAME=personal \
  --build-arg INCLUDE_FRONTEND=true \
  -f deploy/docker/Dockerfile.dist .

# 실행
docker run -d --name centag \
  --env-file config/secrets/.env \
  -e CENTAG_EDITION=personal \
  -e LLM_PROXY_DB_DRIVER=sqlite \
  -e SQLITE_PATH=/app/storage/centag.db \
  -e LLM_PROXY_LOG_OUTPUT=both \
  -e LLM_PROXY_LOG_FORMAT=console \
  -p 20060:20060 \
  -v $(pwd)/deploy/docker/data/storage:/app/storage \
  -v $(pwd)/deploy/docker/data/logs:/app/logs \
  centag-personal:latest

# 중지 및 삭제
docker stop centag && docker rm centag
```

</details>
</details>

---

## 스크린샷

<p align="center">
  <strong>대시보드</strong><br/>
  <img src="docs/assets/readme/screenshot-dashboard.png" alt="대시보드" width="900" />
</p>

<p align="center">
  <strong>파이프라인 시각 편집기</strong><br/>
  <img src="docs/assets/readme/screenshot-pipeline-visual-editor.png" alt="파이프라인 시각 편집기" width="900" />
</p>

<p align="center">
  <strong>Agent 설정</strong><br/>
  <img src="docs/assets/readme/screenshot-agent-config.png" alt="Agent 설정" width="900" />
</p>

<p align="center">
  <strong>Token 사용량 및 과금</strong><br/>
  <img src="docs/assets/readme/screenshot-token-usage.png" alt="Token 사용량 및 과금" width="900" />
</p>

---

## 프록시 모드 — 바로 사용

시나리오별 파이프라인 템플릿 내장 (`#` 단축키로 전환):

| 모드 | 단축키 | 설명 |
|------|--------|------|
| 스마트 스케줄링 | (기본) | 모델 호환성·백엔드 부하 기반 지능형 라우팅 |
| 투명 프록시 | `#t` | 그대로 전달——고성능 무손실, system prompt 미주입 |
| 직결 백엔드 | `#d` | 고정 출구 + 관리형 system prompt |
| 장애 대응 | `#f` | 백엔드 간 자동 디그레이드 |
| 라우팅 | `#r` | 의도 인식 다중 분기 (시나리오 / 모델 자동 전환) |
| 감사 | `#a` | 생성 → 품질 감사 → 피드백 |
| 최적화 | `#o` | 생성 → 콘텐츠 최적화 |
| 집계 | `#ag` | 병렬 다중 모델 생성 → 결과 병합 |
| 보안 방화벽 | `#sec` | 안전 심사 → 생성 → PII 마스킹 |
| RAG 게이트웨이 | `#rag` | 캐시 우선 검색 증강 생성 |
| 지리 라우팅 | `#geo` | 규칙 기반 리전 라우팅 |
| Pi Agent | `#pi` | 코드 작업 → 샌드박스; Q&A → LLM |
| CI/CD Webhook | — | 외부 시스템에서 파이프라인 트리거 |

진짜 하이라이트는 **커스텀 파이프라인**——캔버스에서 자신만의 DAG를 설계하는 것입니다.

---

## 문서

| 주제 | 링크 |
|------|------|
| 전체 문서 색인 | [docs/README.md](docs/README.md) |
| 파이프라인 플러그인 표준 | [docs/guide/pipeline-plugin-standard.md](docs/guide/pipeline-plugin-standard.md) |
| Processor 플러그인 가이드 | [docs/guide/processor-plugins.md](docs/guide/processor-plugins.md) |
| 파이프라인 변수 레퍼런스 | [docs/guide/pipeline-variables.md](docs/guide/pipeline-variables.md) |
| 프록시 모드 | [docs/guide/proxy-modes.md](docs/guide/proxy-modes.md) |
| 백엔드 설정 | [docs/guide/backend-configuration.md](docs/guide/backend-configuration.md) |
| 로컬 프록시 / wrap | [docs/guide/system-proxy-egress.md](docs/guide/system-proxy-egress.md) |
| 환경 변수 | [docs/guide/environment-variables.md](docs/guide/environment-variables.md) |
| API 레퍼런스 | [docs/api/API_REFERENCE.md](docs/api/API_REFERENCE.md) |
| 아키텍처 | [docs/architecture/](docs/architecture/) |
| 보안 | [docs/security/](docs/security/) |

---

## 피드백 및 지원

질문·제안: [GitHub Issues](https://github.com/atoml-ai/centag/issues) 또는 **centag@atoml.com**.

---

## 기여하기

개발자 여러분의 참여를 환영합니다. 버그 수정, 기능 추가, 문서 개선, Agent 적응 확대 등 [Pull Request](https://github.com/atoml-ai/centag/pulls) 또는 [Issues](https://github.com/atoml-ai/centag/issues)로 Centag를 함께 만들어가 주세요.

---

## 라이선스

MIT License (오픈소스 에디션: `minimal` / `personal`)
