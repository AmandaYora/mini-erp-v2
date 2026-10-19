-- id: Q-USR-1
-- severity: blocker
-- rule: stmtrows:1>0
-- action: username ganda; hash non-bcrypt dan tanpa peran dilaporkan
SELECT LOWER(username) u,COUNT(*) FROM {users} WHERE {co}=@c GROUP BY 1 HAVING COUNT(*)>1;
SELECT SUBSTRING(password_hash,1,4) pref,status,COUNT(*) FROM {users} WHERE {co}=@c GROUP BY 1,2;
SELECT u.id_user,u.username FROM {users} u WHERE u.{co}=@c AND NOT EXISTS(SELECT 1 FROM {user_roles} r WHERE r.id_user=u.id_user);
