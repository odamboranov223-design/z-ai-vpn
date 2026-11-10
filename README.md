# Z AI VPN

Windows-клиент VPN с **полностью монохромным** интерфейсом и **нативным frameless
окном**. Никакого CMD-окна при запуске, никаких внешних `xray.exe`, `geoip.dat` —
движок Xray-core встроен прямо в исполняемый файл, интерфейс тоже.

## Что нового в v1.1

- **Бескаркасное окно** — стандартный title bar убран через Win32 (`SetWindowLongPtrW`
  + `WS_CAPTION` clean), нарисован свой в HTML с drag-областью и кнопками
  свернуть/развернуть/закрыть
- **Скруглённые углы окна** — `DwmSetWindowAttribute(DWMWCP_ROUND)` (Windows 11)
- **Тёмная non-client область** — `DWMWA_USE_IMMERSIVE_DARK_MODE` для чистых краёв
- **Без CMD-окна** — линкер-флаг `-H windowsgui` переводит подсистему в GUI
- **Анимированные обои** — 4 фона на выбор в настройках:
  - Aurora — мягкие дрейфующие световые поля (CSS-only, blur 50px)
  - Particles — точки с соединительными линиями (canvas, 120 частиц)
  - Grid — перспективная сетка с радиальной маской (CSS-only)
  - Starfield — мерцающие звёзды с дифракционными крестами (canvas, 260 звёзд)
  - Off — без обоев
- **Полупрозрачные панели** с `backdrop-filter: blur()` — обои проступают сквозь UI
- **JS↔Go биндинги** — `windowMinimize`, `windowToggleMax`, `windowClose`,
  `windowDrag` через WebView2 `Bind()` + Win32 `SendMessage(WM_NCLBUTTONDOWN, HTCAPTION)`

## Что умеет (унаследовано от v1.0)

- **Подписка по ссылке**: base64-список, обычный список, JSON-конфиги Xray
- **Импорт из буфера**: `vless://`, `vmess://`, `trojan://`, `ss://`
- **Транспорты**: TCP, WebSocket, gRPC, XHTTP, HTTPUpgrade, mKCP
- **Защита**: TLS, REALITY, uTLS-отпечатки
- **Пинг серверов**, проверка публичного IP после подключения
- **Системный прокси** Windows (HTTP `127.0.0.1:10809`, SOCKS `127.0.0.1:10808`)
- **Российские сайты напрямую** (переключатель)
- Данные в `%APPDATA%\ZAIVPN`

## Сборка

```bash
# Linux → Windows .exe (кросс-компиляция, без CMD-окна)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -H windowsgui" \
  -o ZAIVPN.exe ./cmd/z-ai-vpn
```

Нужен Go 1.27+. Флаг `-H windowsgui` критичен — без него при запуске открывается
CMD-окно. Готовые сборки лежат в **Releases**.

## Использование

```
ZAIVPN.exe                      # открыть окно
ZAIVPN.exe -add "vless://..."   # добавить сервер и выйти
ZAIVPN.exe -sub "https://..."   # добавить подписку и выйти
ZAIVPN.exe -no-window           # без окна, только локальный адрес
```

Если на машине нет WebView2 Runtime, окно не откроется — приложение покажет
локальный адрес, который нужно открыть в браузере.

## Устройство

```
cmd/z-ai-vpn         точка входа, разбор аргументов, локальный HTTP-сервер
internal/links         разбор ссылок (vless/vmess/trojan/ss/hysteria2/tuic)
internal/xraycfg       сборка JSON-конфигурации Xray
internal/engine        встроенный Xray-core: старт, стоп, журнал
internal/app           сервис: хранилище, движок, системный прокси, подписки
internal/ui            HTTP-API и встроенные ассеты интерфейса (HTML/CSS/JS)
internal/window        нативное окно WebView2: frameless + rounded corners
                        + JS-биндинги для управления окном (Win32 API)
internal/store         сохранение состояния в %APPDATA%\ZAIVPN
internal/country       определение страны по названию сервера
internal/subscription  загрузка подписок
internal/sysproxy      системный прокси Windows
internal/netinfo       пинг и определение внешнего IP
```

## Win32 API (internal/window)

| Вызов | Назначение |
|---|---|
| `GetWindowLongPtrW(GWL_STYLE)` | Читаем стиль окна |
| `SetWindowLongPtrW(GWL_STYLE, …)` | Убираем `WS_CAPTION` (title bar) |
| `DwmSetWindowAttribute(DWMWA_USE_IMMERSIVE_DARK_MODE)` | Тёмные края окна |
| `DwmSetWindowAttribute(DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_ROUND)` | Скруглённые углы (Win11) |
| `SetWindowPos(SWP_FRAMECHANGED)` | Применить новый frame |
| `ShowWindowAsync(SW_MINIMIZE/SW_MAXIMIZE/SW_RESTORE)` | Управление окном из JS |
| `SendMessageW(WM_NCLBUTTONDOWN, HTCAPTION)` | Запуск нативного drag |
| `IsZoomed` | Проверка maximized состояния |
| `WebView2.Bind()` | Экспорт Go-функций в JS |

## Дизайн

- **Палитра**: pure monochrome — `#050505` / `#0a0a0a` / `#161616` / `#f5f5f5`
- **Шрифты**: Inter для UI, JetBrains Mono для технических данных
- **Power-dial**: круглый SVG-тумблер с дугой статуса
- **Connection Orb** 200px: 3 вращающихся кольца + пульсирующее ядро
- **Ping-bars**: 5 вертикальных полосок вместо чисел
- **Country codes**: `DE`, `NL`, `US` моноширинным шрифтом
- **Status-strip**: внизу — IP, таймер сессии, текущий сервер, версия
- **Title bar**: кастомный, с брендом и статус-пиллем
- **Анимированные обои** проступают сквозь полупрозрачные панели

## Проверка

```bash
go vet ./...
go test ./...
```

## Ограничения

- Скруглённые углы окна работают только на Windows 11 (на Windows 10 — квадратные)
- `hysteria2://` и `tuic://` парсятся, но встроенный движок Xray их не умеет
- Полные базы `geosite`/`geoip` не встроены
