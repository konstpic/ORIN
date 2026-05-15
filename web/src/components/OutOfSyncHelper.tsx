import { AlertCircle, BookOpen, Copy, Check } from "lucide-react";
import { useState } from "react";

interface OutOfSyncHelperProps {
  appName: string;
  outOfSyncCount: number;
}

const commonIgnoreRules = [
  {
    title: "HPA-managed replicas",
    description: "Ignore replica count managed by HorizontalPodAutoscaler",
    yaml: `ignoreDifferences:
  - group: apps
    kind: Deployment
    jsonPointers:
      - /spec/replicas`,
  },
  {
    title: "Service cluster IPs",
    description: "Ignore cluster-assigned Service IPs",
    yaml: `ignoreDifferences:
  - group: ""
    kind: Service
    jsonPointers:
      - /spec/clusterIP
      - /spec/clusterIPs`,
  },
  {
    title: "PVC volume binding",
    description: "Ignore PersistentVolumeClaim volume name",
    yaml: `ignoreDifferences:
  - group: ""
    kind: PersistentVolumeClaim
    jsonPointers:
      - /spec/volumeName`,
  },
  {
    title: "ServiceAccount token secrets",
    description: "Ignore auto-populated token data",
    yaml: `ignoreDifferences:
  - group: ""
    kind: Secret
    jsonPointers:
      - /data/token
      - /data/ca.crt
      - /data/namespace`,
  },
];

export function OutOfSyncHelper({ appName, outOfSyncCount }: OutOfSyncHelperProps) {
  const [expanded, setExpanded] = useState(false);
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  const handleCopy = async (text: string, index: number) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedIndex(index);
      setTimeout(() => setCopiedIndex(null), 2000);
    } catch (err) {
      console.error("Failed to copy:", err);
    }
  };

  if (outOfSyncCount === 0) return null;

  return (
    <div className="rounded-lg border border-amber-500/40 bg-amber-500/5 overflow-hidden">
      <button
        type="button"
        className="w-full px-4 py-3 flex items-start gap-3 hover:bg-amber-500/10 transition-colors text-left"
        onClick={() => setExpanded(!expanded)}
      >
        <AlertCircle className="size-5 shrink-0 text-amber-400 mt-0.5" strokeWidth={2} />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold text-amber-200 mb-1">
            {outOfSyncCount} {outOfSyncCount === 1 ? "resource" : "resources"} out of sync
          </div>
          <div className="text-xs text-amber-300/80 leading-relaxed">
            This may be caused by cluster-managed fields (HPA replicas, Service IPs, etc.). 
            Click to see common solutions.
          </div>
        </div>
        <span className={`text-amber-300 transition-transform ${expanded ? "rotate-90" : ""}`}>▶</span>
      </button>

      {expanded && (
        <div className="border-t border-amber-500/30 bg-amber-500/5">
          <div className="px-4 py-3 space-y-4">
            <div className="text-xs text-amber-200/90 leading-relaxed">
              <p className="mb-2">
                <strong>Common causes of persistent OutOfSync status:</strong>
              </p>
              <ul className="list-disc list-inside space-y-1 text-amber-300/80">
                <li>HPA (HorizontalPodAutoscaler) managing replica counts</li>
                <li>Cluster-assigned Service IPs and PVC volume names</li>
                <li>Annotations added by controllers or admission webhooks</li>
                <li>CRD default values applied server-side</li>
              </ul>
            </div>

            <div className="space-y-3">
              <div className="text-xs font-semibold text-amber-200 uppercase tracking-wide">
                Quick Fixes
              </div>
              {commonIgnoreRules.map((rule, index) => (
                <div
                  key={index}
                  className="rounded border border-amber-500/30 bg-amber-500/10 p-3"
                >
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <div>
                      <div className="text-xs font-semibold text-amber-200">{rule.title}</div>
                      <div className="text-xs text-amber-300/70 mt-0.5">{rule.description}</div>
                    </div>
                    <button
                      type="button"
                      className="shrink-0 p-1.5 rounded hover:bg-amber-500/20 text-amber-300 hover:text-amber-200 transition-colors"
                      onClick={() => handleCopy(rule.yaml, index)}
                      title="Copy to clipboard"
                    >
                      {copiedIndex === index ? (
                        <Check className="size-4" strokeWidth={2} />
                      ) : (
                        <Copy className="size-4" strokeWidth={2} />
                      )}
                    </button>
                  </div>
                  <pre className="text-xs font-mono text-amber-100/90 bg-black/20 rounded p-2 overflow-x-auto">
                    {rule.yaml}
                  </pre>
                </div>
              ))}
            </div>

            <div className="pt-3 border-t border-amber-500/30">
              <a
                href="/docs/troubleshooting-out-of-sync.md"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 text-xs text-amber-200 hover:text-amber-100 underline"
              >
                <BookOpen className="size-4" />
                Read full troubleshooting guide
              </a>
            </div>

            <div className="text-xs text-amber-300/70 leading-relaxed">
              <strong>Next steps:</strong>
              <ol className="list-decimal list-inside mt-1 space-y-1">
                <li>Click the <strong>Diff</strong> button to see which fields differ</li>
                <li>Add appropriate <code className="px-1 py-0.5 bg-black/20 rounded">ignoreDifferences</code> rules to your Application</li>
                <li>Wait 30 seconds or click <strong>Refresh</strong> to verify</li>
              </ol>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
