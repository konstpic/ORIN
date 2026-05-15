/** Best-effort removal of metadata.managedFields blocks from YAML for display. */
export function stripManagedFieldsYaml(yaml: string): string {
  const lines = yaml.split("\n");
  const out: string[] = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const m = line.match(/^(\s*)managedFields:\s*$/);
    if (m) {
      const indent = m[1].length;
      i++;
      while (i < lines.length) {
        const next = lines[i];
        if (next.trim() === "") {
          i++;
          continue;
        }
        const indM = next.match(/^(\s*)/);
        const nextIndent = indM ? indM[1].length : 0;
        if (nextIndent <= indent) {
          i--;
          break;
        }
        i++;
      }
      continue;
    }
    out.push(line);
  }
  return out.join("\n");
}
