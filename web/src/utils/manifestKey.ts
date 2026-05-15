/** Matches server controller key(): group/Kind/namespace/name */
export function manifestResourceKey(raw: Record<string, unknown>): string {
  const apiVersion = String(raw.apiVersion ?? "");
  let group = "";
  if (apiVersion.includes("/")) {
    group = apiVersion.split("/")[0] ?? "";
  }
  const kind = String(raw.kind ?? "?");
  const meta = (raw.metadata as Record<string, unknown>) ?? {};
  const ns = String(meta.namespace ?? "");
  const name = String(meta.name ?? "?");
  return `${group}/${kind}/${ns}/${name}`;
}

export function diffResourceKey(d: { group: string; kind: string; namespace?: string; name: string }): string {
  return `${d.group}/${d.kind}/${d.namespace ?? ""}/${d.name}`;
}
