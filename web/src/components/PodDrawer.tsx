import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Boxes, Download, Pause, Play, RefreshCcw, Search } from "lucide-react";
import { api } from "../api/client";
import { useAuth } from "../state/auth";
import { HealthBadge } from "./Badges";
import type { HealthStatus, PodEvent, ResourceNode } from "../api/types";
import { iconForKind, kindIconTileClass } from "../k8s/kindMeta";
import { useAnimatedList } from "../hooks/useAnimatedList";

type Tab = "logs" | "terminal" | "events";

function eventRowClass(ev: PodEvent): string {
  if (ev.type === "Warning") return "bg-amber-500/5 hover:bg-amber-500/10";
  return "hover:bg-[var(--color-accent-muted)]/15";
}

function getCategoryBadgeColor(category?: string): string {
  if (!category) return "bg-gray-500/20 text-gray-300";
  
  if (category.includes("Crash") || category.includes("Failed") || category.includes("Error")) 
    return "bg-red-500/20 text-red-300";
  if (category.includes("Probe")) 
    return "bg-orange-500/20 text-orange-300";
  if (category.includes("ImagePull")) 
    return "bg-blue-500/20 text-blue-300";
  if (category.includes("Success") || category.includes("Pulled") || category.includes("Started") || category.includes("Mount"))
    return "bg-green-500/20 text-green-300";
  if (category.includes("Starting") || category.includes("Created"))
    return "bg-cyan-500/20 text-cyan-300";
  if (category.includes("Stopping"))
    return "bg-purple-500/20 text-purple-300";
  
  return "bg-gray-500/20 text-gray-300";
}

function formatEventTime(t: string | null | undefined): string {
  if (!t) return "—";
  try {
    return new Date(t).toLocaleString(undefined, { dateStyle: "short", timeStyle: "medium" });
  } catch {
    return t;
  }
}

export function PodDrawer({
  appName,
  node,
  onClose,
}: {
  appName: string;
  node: ResourceNode;
  onClose: () => void;
}) {
  const token = useAuth((s) => s.token);
  const [tab, setTab] = useState<Tab>("logs");
  const [container, setContainer] = useState("");
  const [shell, setShell] = useState("/bin/sh");
  const [shellLoading, setShellLoading] = useState(false);
  const [tailLines, setTailLines] = useState(800);
  const [followLogs, setFollowLogs] = useState(false);
  const [logFilter, setLogFilter] = useState("");
  const [eventFilter, setEventFilter] = useState<"all" | "warning">("all");
  const [logsText, setLogsText] = useState<string>("");
  const [logsError, setLogsError] = useState<string | null>(null);
  const [termError, setTermError] = useState<string | null>(null);
  const [termConnecting, setTermConnecting] = useState(false);
  const [termGen, setTermGen] = useState(0);

  const pod = node.name;

  const { data: summary } = useQuery({
    queryKey: ["pod", appName, pod],
    queryFn: () => api.getPod(appName, pod),
    retry: 0,
  });

  useEffect(() => {
    const list = summary?.containers ?? [];
    if (list.length && !list.find((c) => c.name === container)) {
      setContainer(list[0].name);
    }
  }, [summary, container]);

  // Auto-detect shell when container changes
  useEffect(() => {
    if (!container) return;
    setShellLoading(true);
    void (async () => {
      try {
        const result = await api.getPodShell(appName, pod, container);
        setShell(result.shell);
      } catch {
        setShell("/bin/sh"); // fallback
      } finally {
        setShellLoading(false);
      }
    })();
  }, [container, appName, pod]);

  const {
    data: events,
    isLoading: eventsLoading,
    error: eventsError,
    refetch: refetchEvents,
  } = useQuery({
    queryKey: ["pod-events", appName, pod],
    queryFn: () => api.getPodEvents(appName, pod),
    enabled: tab === "events",
    refetchInterval: tab === "events" ? 5000 : false,
    retry: 0,
  });

  const filteredEvents = useMemo(() => {
    const list = events ?? [];
    if (eventFilter === "warning") return list.filter((e) => e.type === "Warning");
    return list;
  }, [events, eventFilter]);

  const loadLogsOnce = useCallback(async () => {
    if (!container) return;
    setLogsError(null);
    try {
      const text = await api.getPodLog(appName, pod, { container, tailLines });
      setLogsText(text);
    } catch (e) {
      setLogsError((e as Error).message || "Failed to load logs");
    }
  }, [appName, pod, container, tailLines]);

  useEffect(() => {
    if (tab !== "logs" || !container) return;
    if (followLogs) return; // streaming handled below
    void loadLogsOnce();
  }, [tab, container, followLogs, loadLogsOnce]);

  useEffect(() => {
    if (tab !== "logs" || !container || !followLogs) return;
    setLogsText("");
    setLogsError(null);
    const ctrl = new AbortController();
    let cancelled = false;

    (async () => {
      try {
        const q = new URLSearchParams();
        q.set("container", container);
        q.set("tailLines", String(tailLines));
        q.set("follow", "true");
        const res = await fetch(
          `/api/v1/applications/${encodeURIComponent(appName)}/pods/${encodeURIComponent(pod)}/log?${q}`,
          { credentials: "include", signal: ctrl.signal },
        );
        if (!res.ok || !res.body) {
          const t = await res.text().catch(() => "");
          setLogsError(`HTTP ${res.status}${t ? `: ${t}` : ""}`);
          return;
        }
        const reader = res.body.getReader();
        const dec = new TextDecoder();
        while (!cancelled) {
          const { value, done } = await reader.read();
          if (done) break;
          if (value && value.length > 0) {
            setLogsText((prev) => prev + dec.decode(value, { stream: true }));
          }
        }
      } catch (e) {
        if (!ctrl.signal.aborted) setLogsError((e as Error).message || "Stream error");
      }
    })();

    return () => {
      cancelled = true;
      ctrl.abort();
    };
  }, [tab, container, followLogs, tailLines, appName, pod]);

  const filteredLogs = useMemo(() => {
    if (!logFilter.trim()) return logsText;
    const needle = logFilter.toLowerCase();
    return logsText
      .split("\n")
      .filter((line) => line.toLowerCase().includes(needle))
      .join("\n");
  }, [logsText, logFilter]);

  const termHostRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (tab !== "terminal" || !container || !termHostRef.current) return;
    setTermError(null);
    setTermConnecting(true);

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      scrollback: 5000,
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
      theme: { background: "#0a0e14", foreground: "#c9d1d9" },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(termHostRef.current);
    try { fit.fit(); } catch { /* host may not be sized yet */ }

    const proto = location.protocol === "https:" ? "wss" : "ws";
    const qs = new URLSearchParams();
    qs.set("container", container);
    qs.set("command", shell);
    if (token) qs.set("token", token);
    const url = `${proto}://${location.host}/api/v1/applications/${encodeURIComponent(appName)}/pods/${encodeURIComponent(pod)}/exec?${qs}`;
    const ws = new WebSocket(url);
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      setTermConnecting(false);
      term.reset();
      term.writeln(`\x1b[36mConnected\x1b[0m — ${shell}.  (Ctrl+D to exit on the pod side)\r\n`);
    };

    ws.onclose = (ev) => {
      setTermConnecting(false);
      const why = ev.reason || (ev.code === 1006 ? "abnormal close" : `code ${ev.code}`);
      term.writeln(`\r\n\x1b[33mDisconnected\x1b[0m — ${why}`);
    };

    ws.onmessage = (ev) => {
      if (typeof ev.data === "string") {
        try {
          const j = JSON.parse(ev.data) as { error?: string };
          if (j.error) {
            setTermError(j.error);
            term.writeln(`\r\n\x1b[31m${j.error}\x1b[0m`);
          }
        } catch {
          /* ignore */
        }
        return;
      }
      const buf = new Uint8Array(ev.data as ArrayBuffer);
      if (buf.length === 0) return;
      const kind = buf[0];
      const payload = buf.slice(1);
      const text = new TextDecoder().decode(payload);
      if (kind === 1 || kind === 2) term.write(text);
    };

    ws.onerror = () => {
      setTermConnecting(false);
      setTermError("WebSocket error — see browser devtools network tab.");
      term.writeln("\r\n\x1b[31mWebSocket error\x1b[0m");
    };

    const sub = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data));
      }
    });

    const el = termHostRef.current;
    const ro = new ResizeObserver(() => {
      try { fit.fit(); } catch { /* ignore */ }
    });
    ro.observe(el);

    return () => {
      ro.disconnect();
      sub.dispose();
      try { ws.close(); } catch { /* ignore */ }
      term.dispose();
    };
  }, [tab, container, appName, pod, token, shell, termGen]);

  return (
    <aside className="w-full h-full flex flex-col rounded-none border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl overflow-hidden">
      <div className="flex items-start justify-between gap-2 border-b border-[var(--color-border)] px-4 py-3 bg-[var(--color-surface-muted)]">
        <div className="min-w-0">
          <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">Pod</div>
          <div className="font-semibold text-[var(--color-text)] truncate">{pod}</div>
          <div className="text-xs text-[var(--color-text-muted)] truncate">{node.namespace}</div>
          <div className="mt-1 flex items-center gap-2">
            <HealthBadge status={node.health as HealthStatus} size="sm" />
            {summary?.phase && (
              <span className="text-xs font-mono text-[var(--color-text-muted)]">{summary.phase}</span>
            )}
          </div>
        </div>
        <button
          type="button"
          className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1 text-xs text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
          onClick={onClose}
        >
          Close
        </button>
      </div>

      <div className="px-4 py-2 border-b border-[var(--color-border)] flex flex-wrap gap-x-3 gap-y-2 items-end">
        <div className="flex-1 min-w-[160px]">
          <label className="block text-[10px] uppercase font-semibold text-[var(--color-text-muted)] mb-1">Container</label>
          <select
            className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-sm text-[var(--color-text)]"
            value={container}
            onChange={(e) => setContainer(e.target.value)}
          >
            {(summary?.containers ?? []).map((c) => (
              <option key={c.name} value={c.name}>
                {c.name}
              </option>
            ))}
            {(summary?.initContainers ?? []).map((c) => (
              <option key={`init-${c.name}`} value={c.name}>
                {c.name} (init)
              </option>
            ))}
          </select>
        </div>
        {tab === "terminal" && (
          <div className="min-w-[150px]">
            <label className="block text-[10px] uppercase font-semibold text-[var(--color-text-muted)] mb-1">Shell</label>
            <div className="flex gap-1 items-center">
              <div className="flex-1 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-sm text-[var(--color-text)] font-mono">
                {shellLoading ? "Detecting..." : shell}
              </div>
              <button
                type="button"
                title="Reconnect terminal"
                className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
                onClick={() => setTermGen((g) => g + 1)}
              >
                <RefreshCcw className="size-3.5" />
              </button>
            </div>
          </div>
        )}
        {tab === "logs" && (
          <div className="min-w-[120px]">
            <label className="block text-[10px] uppercase font-semibold text-[var(--color-text-muted)] mb-1">Tail</label>
            <select
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-sm text-[var(--color-text)]"
              value={tailLines}
              onChange={(e) => setTailLines(parseInt(e.target.value, 10))}
            >
              {[200, 500, 800, 2000, 5000].map((n) => (
                <option key={n} value={n}>{n} lines</option>
              ))}
            </select>
          </div>
        )}
      </div>

      <div className="flex border-b border-[var(--color-border)] px-2 overflow-x-auto">
        {(["logs", "events", "terminal"] as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            className={`px-3 py-2 text-sm font-medium border-b-2 -mb-px shrink-0 capitalize transition-all duration-150 hover:translate-y-[-1px] ${
              tab === t
                ? "border-[var(--color-accent)] text-[var(--color-accent)]"
                : "border-transparent text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            }`}
            onClick={() => setTab(t)}
          >
            {t}
          </button>
        ))}
      </div>

      <div className="flex-1 min-h-0 flex flex-col">
        {tab === "logs" && (
          <div className="flex-1 flex flex-col min-h-[300px] p-3 gap-2">
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                className="inline-flex items-center gap-1.5 rounded-md bg-[var(--color-accent)] px-2.5 py-1.5 text-xs font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
                onClick={() => void loadLogsOnce()}
                disabled={!container || followLogs}
                title="Re-fetch tail"
              >
                <RefreshCcw className="size-3.5" /> Refresh
              </button>
              <button
                type="button"
                className={`inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs font-medium ${
                  followLogs
                    ? "border-[var(--color-border-strong)] bg-[var(--color-accent-muted)] text-[var(--color-accent)]"
                    : "border-[var(--color-border)] bg-[var(--color-input-bg)] text-[var(--color-text)]"
                }`}
                onClick={() => setFollowLogs((v) => !v)}
                disabled={!container}
              >
                {followLogs ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
                {followLogs ? "Streaming" : "Follow"}
              </button>
              <a
                className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
                href={`/api/v1/applications/${encodeURIComponent(appName)}/pods/${encodeURIComponent(pod)}/log?container=${encodeURIComponent(container)}&tailLines=${tailLines}`}
                download={`${pod}-${container}.log`}
                target="_blank"
                rel="noreferrer"
              >
                <Download className="size-3.5" /> Download
              </a>
              <div className="relative flex-1 min-w-[140px]">
                <Search className="size-3.5 absolute left-2 top-1/2 -translate-y-1/2 text-[var(--color-text-muted)] pointer-events-none" />
                <input
                  type="search"
                  placeholder="Filter lines…"
                  value={logFilter}
                  onChange={(e) => setLogFilter(e.target.value)}
                  className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] pl-7 pr-2 py-1.5 text-xs text-[var(--color-text)]"
                />
              </div>
            </div>
            {logsError && (
              <div className="rounded-md border border-red-500/30 bg-red-500/10 px-2.5 py-1.5 text-xs text-red-300">
                {logsError}
              </div>
            )}
            <pre className="flex-1 overflow-auto rounded-md border border-[var(--color-border)] bg-[#05070a] text-[var(--color-text)] p-3 text-xs font-mono whitespace-pre-wrap">
              {filteredLogs || (logsError ? "" : "—")}
            </pre>
          </div>
        )}
        {tab === "events" && (
          <div className="flex-1 flex flex-col min-h-[300px] p-3 gap-2">
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                className="inline-flex items-center gap-1.5 rounded-md bg-[var(--color-accent)] px-2.5 py-1.5 text-xs font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
                onClick={() => void refetchEvents()}
                disabled={eventsLoading}
              >
                <RefreshCcw className="size-3.5" /> Refresh
              </button>
              <div className="inline-flex rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-0.5">
                {(["all", "warning"] as const).map((opt) => (
                  <button
                    key={opt}
                    type="button"
                    className={`rounded-sm px-2 py-1 text-xs font-medium capitalize ${
                      eventFilter === opt
                        ? "bg-[var(--color-surface)] text-[var(--color-accent)] border border-[var(--color-border)]"
                        : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                    }`}
                    onClick={() => setEventFilter(opt)}
                  >
                    {opt}
                  </button>
                ))}
              </div>
              <span className="text-[10px] text-[var(--color-text-muted)] ml-auto">
                Auto-refresh every 5 s
              </span>
            </div>
            {eventsError && (
              <div className="rounded-md border border-red-500/30 bg-red-500/10 px-2.5 py-1.5 text-xs text-red-300">
                {(eventsError as Error).message}
              </div>
            )}
            <div className="flex-1 overflow-auto rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)]">
              {!filteredEvents.length && !eventsLoading && (
                <div className="p-4 text-xs text-[var(--color-text-muted)]">
                  {eventFilter === "warning" ? "No warnings." : "No events for this pod."}
                </div>
              )}
              {!!filteredEvents.length && (
                <table className="w-full text-left text-xs">
                  <thead className="sticky top-0 bg-[var(--color-surface)] border-b border-[var(--color-border)] z-10">
                    <tr className="text-[var(--color-text-muted)] uppercase tracking-wide">
                      <th className="px-2 py-2 w-16">Type</th>
                      <th className="px-2 py-2 w-28">Category</th>
                      <th className="px-2 py-2 w-28">Reason</th>
                      <th className="px-2 py-2">Message</th>
                      <th className="px-2 py-2 w-40 hidden md:table-cell">Last seen</th>
                      <th className="px-2 py-2 w-10 text-right">#</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] text-[var(--color-text)]">
                    {filteredEvents.map((ev, i) => (
                      <tr key={`${ev.reason}-${i}`} className={`align-top ${eventRowClass(ev)}`}>
                        <td className={`px-2 py-1.5 whitespace-nowrap font-mono ${ev.type === "Warning" ? "text-amber-300" : ""}`}>{ev.type}</td>
                        <td className="px-2 py-1.5 whitespace-nowrap">
                          {ev.category && (
                            <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-semibold ${getCategoryBadgeColor(ev.category)}`}>
                              {ev.category}
                            </span>
                          )}
                        </td>
                        <td className="px-2 py-1.5 whitespace-nowrap text-[var(--color-text-muted)]">{ev.reason}</td>
                        <td className="px-2 py-1.5 break-words">{ev.message}</td>
                        <td className="px-2 py-1.5 text-[var(--color-text-muted)] hidden md:table-cell whitespace-nowrap">
                          {formatEventTime(ev.lastTime ?? ev.firstTime)}
                        </td>
                        <td className="px-2 py-1.5 text-right text-[var(--color-text-muted)]">{ev.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        )}
        {tab === "terminal" && (
          <div className="flex-1 min-h-[320px] p-3 flex flex-col gap-2">
            {termError && (
              <div className="rounded-md border border-red-500/30 bg-red-500/10 px-2.5 py-1.5 text-xs text-red-300">
                {termError}. Try a different shell from the dropdown above.
              </div>
            )}
            {termConnecting && !termError && (
              <div className="rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)] px-2.5 py-1.5 text-xs text-[var(--color-text-muted)]">
                Connecting…
              </div>
            )}
            <div ref={termHostRef} className="flex-1 min-h-[280px] rounded-md overflow-hidden border border-[var(--color-border)]" />
          </div>
        )}
      </div>
    </aside>
  );
}

export function PodGroupSideCard({
  node,
  appName,
  onSelectPod,
  onExpandOnMap,
  onClose,
}: {
  node: ResourceNode;
  appName: string;
  onSelectPod: (n: ResourceNode) => void;
  onExpandOnMap: () => void;
  onClose: () => void;
}) {
  const pods = node.groupedPods ?? [];
  const animatedPods = useAnimatedList(pods, (p) => p.uid);
  const Icon = iconForKind("PodGroup");
  const parentKind = node.groupParentKind === "Deployment" ? "Deployment" : "ReplicaSet";
  return (
    <aside className="w-full flex flex-col max-h-[min(88vh,900px)] rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl overflow-hidden">
      <div className="flex items-start justify-between gap-2 border-b border-[var(--color-border)] px-4 py-3 bg-[var(--color-surface-muted)]">
        <div className="min-w-0 flex gap-3">
          <span
            className={`inline-flex shrink-0 items-center justify-center rounded-xl size-11 [&_svg]:size-5 ${kindIconTileClass("PodGroup")}`}
          >
            <Icon strokeWidth={1.65} />
          </span>
          <div>
            <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">Pod group</div>
            <div className="font-semibold text-[var(--color-text)]">{pods.length} pods</div>
            <div className="text-xs text-[var(--color-text-muted)] mt-0.5">Application: {appName}</div>
            <div className="mt-1.5">
              <HealthBadge status={node.health as HealthStatus} size="sm" />
            </div>
          </div>
        </div>
        <button
          type="button"
          className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1 text-xs text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
          onClick={onClose}
        >
          Close
        </button>
      </div>

      <div className="px-4 py-3 border-b border-[var(--color-border)]">
        <button
          type="button"
          className="w-full rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-accent-muted)] px-3 py-2.5 text-sm font-medium text-[var(--color-accent)] hover:brightness-110"
          onClick={onExpandOnMap}
        >
          Show all pods on map
        </button>
        <p className="mt-2 text-xs text-[var(--color-text-muted)]">
          Expands this {parentKind} on the topology so every pod is its own node (can be heavy with many replicas).
        </p>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto px-4 py-3">
        <div className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide mb-2">Pods</div>
        <ul className="space-y-1">
          {animatedPods.map(({ item: p, key, status }) => (
            <li
              key={key}
              className={
                status === "entering" ? "pod-row-enter" : status === "exiting" ? "pod-row-exit" : ""
              }
            >
              <button
                type="button"
                className="w-full flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)] px-3 py-2 text-left text-sm text-[var(--color-text)] hover:border-[var(--color-border-strong)] disabled:pointer-events-none"
                onClick={() => onSelectPod(p)}
                disabled={status === "exiting"}
              >
                <Boxes className="size-4 shrink-0 text-[var(--color-accent)]" strokeWidth={1.75} />
                <span className="truncate font-mono text-xs">{p.name}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </aside>
  );
}
