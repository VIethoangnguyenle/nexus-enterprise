-- Seed data: Global NGAC infrastructure + sample workspace

-- Global Policy Class (for cross-workspace sharing)
INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES
  ('pc-global', 'PC_Global', 'PC', '{"scope": "global"}'),
  ('ua-public-users', 'PublicUsers', 'UA', '{}'),
  ('oa-public-docs', 'PublicDocs', 'OA', '{}')
ON CONFLICT (id) DO NOTHING;

-- Assignments for global scope
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES
  ('asg-pub-users-global', 'ua-public-users', 'pc-global'),
  ('asg-pub-docs-global', 'oa-public-docs', 'pc-global')
ON CONFLICT (child_id, parent_id) DO NOTHING;

-- PublicUsers can read PublicDocs
INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES
  ('assoc-pub-read', 'ua-public-users', 'oa-public-docs', ARRAY['read'])
ON CONFLICT (ua_id, oa_id) DO UPDATE SET operations = ARRAY['read'];
