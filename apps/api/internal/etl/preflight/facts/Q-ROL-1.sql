-- id: Q-ROL-1
-- severity: info
-- rule: none
-- action: izin/peran legacy
SELECT p.permission_code,COUNT(rp.id_role) n_roles FROM {permissions} p LEFT JOIN {role_permissions} rp USING(id_permission) GROUP BY 1;
SELECT code,{co},is_system_role,COUNT(*) FROM {roles} WHERE {co}=@c OR {co} IS NULL GROUP BY 1,2,3;
