# Centag

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Русский](README.ru.md) | [Español](README.es.md)

코딩 Agent를 **로컬에서 원클릭 프록시 접속**하고, 백엔드와 API Key를 **통합 관리**하며, 시나리오별로 **프록시 동작을 설정**(전환·장애 대응·파이프라인)—도구마다 설정을 반복할 필요가 없습니다.

개인 개발자용: Centag 설치 → wrap 또는 설정 파일로 Agent 연결 → Web에서 백엔드와 정책을 관리.

## 설치

방법 중 하나를 선택하세요. 설치 후 `centag`를 실행하고 **http://localhost:20060** 을 엽니다.

### 방법 1: 원라인 스크립트 (권장, Node.js 불필요)

```bash
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
```

기본 경로 `~/.centag/`, PATH 등록을 시도합니다. 이후 `centag` / `centag wrap` 사용.

### 방법 2: npm (이미 Node.js가 있을 때)

```bash
# 전역 설치 (온라인 패키지, Release에서 바이너리 다운로드)
npm install -g @atomlai/centag

# 전역 경로를 바꾸지 않고 시험
npx --yes @atomlai/centag

# 오프라인 / 폐쇄망 패키지
npm install -g @atomlai/centag-offline
```

`npm install -g` 권한 오류 시 `npx` 또는 위 스크립트를 사용하세요. 자세한 내용: [apps/centag-npm/README.md](apps/centag-npm/README.md).

### 방법 3: Docker (소스)

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # 필요 시 수정
./start.sh docker up                                 # 기본: personal 컨테이너
```

관리 UI는 동일하게 http://localhost:20060. 중지: `./start.sh docker down`.

---

## 설치 후: Agent 연결

목표: Agent는 평소처럼 쓰고, 트래픽만 Centag를 경유(백엔드 공유·장애 대응·계량).

1. **Web 열기** → 백엔드를 하나 이상 추가·활성화(API Key / 로컬 호환 엔드포인트).
2. **Agent 연동**(Web 메뉴)—마법사로 설정 생성/기록. 또는
3. **프로세스 프록시(권장, Agent 설정 변경 최소)**:

```bash
# 로컬 Centag가 떠 있을 때 wrap으로 Agent 실행
centag wrap run -- opencode
# opencode를 본인 Agent 실행 명령으로 교체

# 점검
centag wrap doctor
```

참고: `centag wrap`은 게이트웨이를 **시작하지 않습니다**. 이미 실행 중인 Centag로 Agent 프로세스 트래픽만 보냅니다. 가이드: [시스템 프록시 egress](docs/guide/system-proxy-egress.md).

---

## 왜 Centag인가

| 필요한 것 | Centag가 하는 일 |
|-----------|------------------|
| **백엔드 빠른 전환** | 여러 백엔드 통합 관리; Web에서 활성화/전환. Agent마다 설정을 다시 고치지 않음 |
| **자동 장애 대응 + API 풀** | 여러 Key 로테이션; 제한·장애 시 자동 전환 |
| **시나리오별 파이프라인** | 투명 전달·직결·스케줄링·검수 등 구성 가능; 시나리오 변경 = 정책 변경 |
| **사용량·과금 계량** | Token/비용 추적, 개인 사용량 파악 |

한 줄로: **백엔드와 정책은 하나의 입구, Agent는 코드만 작성.**

## 기능 목록

1. **백엔드 / 모델과 API Key 풀**  
   Web에서 백엔드·모델 설정. 동일 백엔드에서 **여러 API Key를 풀링·로테이션**(제한·장애 시 자동 교체).

2. **파이프라인 시각적 편집**  
   캔버스에서 프록시 동작 커스터마이즈(전달·스케줄·검수 등). 시나리오별 정책 전환, Agent 코드 수정 불필요.

3. **`centag wrap` 비침습 연동**  
   wrap으로 Agent를 실행해 Centag로 트래픽을 넣고, **Agent 자체 설정을 바꾸지 않아도 됨**.

4. **Agent 설정 파일 직접 수정**  
   Agent의 API Base / Key를 Centag로 지정해 일반 LLM 게이트웨이처럼 사용(Web「Agent 연동」마법사가 작성 지원).

둘 중 선택: wrap은 설정 변경이 적고, 설정 파일 방식은 표준 OpenAI 호환 엔드포인트에 맞습니다.

## 스크린샷

| 대시보드 | Agent 연동 |
|----------|------------|
| ![대시보드](docs/assets/readme/dashboard.png) | ![Agent 연동](docs/assets/readme/agent-setup.png) |

## 문서

- [문서 색인](docs/README.md)
- [환경 변수](docs/guide/environment-variables.md)
- [로컬 프록시 / wrap](docs/guide/system-proxy-egress.md)
- [API 레퍼런스](docs/api/API_REFERENCE.md)

## 피드백 및 지원

질문·제안: [GitHub Issues](https://github.com/atoml-ai/centag/issues) 또는 **centag@atoml.com**.

## 라이선스

MIT License (오픈소스 에디션: `minimal` / `personal`)
