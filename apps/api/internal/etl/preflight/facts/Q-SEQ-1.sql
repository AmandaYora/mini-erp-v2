-- id: Q-SEQ-1
-- severity: info
-- rule: none
-- action: counter legacy
SELECT b.code,s.sequence_key,s.prefix,s.current_value FROM {branch_document_sequences} s JOIN {branches} b USING(id_branch) WHERE b.{co}=@c ORDER BY 1,2;
SELECT sequence_key,prefix,current_value FROM {finance_document_sequences} WHERE {co}=@c;
