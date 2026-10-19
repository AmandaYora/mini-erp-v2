// Bentuk JSON backend: GET /api/v1/ops/integrity (bootstrap/ops.go, perm
// stock.update) → {issues:[{kind,docType,docId,number,detail}], count};
// issue = shared/integrity.Issue.

export interface IntegrityIssue {
  kind: string;
  docType: string;
  docId: number;
  number: string;
  detail: string;
}

export interface OpsIntegrityReport {
  issues: IntegrityIssue[];
  count: number;
}
