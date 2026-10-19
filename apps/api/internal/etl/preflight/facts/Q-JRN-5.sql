-- id: Q-JRN-5
-- severity: info
-- rule: none
-- action: akun buatan pengguna dipakai jurnal: dimigrasi dengan kode asli
SELECT a.id_finance_account,a.account_code,a.account_name,a.system_key,COUNT(l.id_finance_journal_line) n_lines
FROM {finance_accounts} a LEFT JOIN {finance_journal_lines} l ON l.id_finance_account=a.id_finance_account WHERE a.{co}=@c GROUP BY a.id_finance_account
HAVING n_lines>0 AND (a.system_key IS NULL OR a.system_key NOT IN ('cash_cash','cash_bank','accounts_receivable','inventory','input_tax','accounts_payable','output_tax','sales_revenue','cogs','opening_equity','customer_advance','supplier_advance','stock_difference','rounding_difference','purchase_return_difference'));
