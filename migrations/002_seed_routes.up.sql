INSERT INTO routes_registry (
    service_name, feature_name, method, downstream_url, transactional, route_class,
    timeout_ms, retry_count, required_scope, is_active, description
) VALUES
('smartbank', 'pembayaran_transaksi', 'POST', 'http://localhost:8101/smartbank/pembayaran_transaksi', TRUE, 'transactional', 5000, 0, 'payment:write', TRUE, 'SmartBank payment endpoint for Gateway fee and ecosystem transactions'),
('marketplace', 'checkout', 'POST', 'http://localhost:8102/marketplace/checkout', TRUE, 'transactional', 5000, 0, 'marketplace:write', TRUE, 'Marketplace checkout route'),
('pos', 'pembayaran', 'POST', 'http://localhost:8103/pos/pembayaran', TRUE, 'transactional', 5000, 0, 'pos:write', TRUE, 'POS payment route'),
('supplierhub', 'pembayaran', 'POST', 'http://localhost:8104/supplierhub/pembayaran', TRUE, 'transactional', 5000, 0, 'supplier:write', TRUE, 'SupplierHub payment route'),
('logistikita', 'pembayaran_logistik', 'POST', 'http://localhost:8105/logistikita/pembayaran_logistik', TRUE, 'transactional', 5000, 0, 'logistics:write', TRUE, 'LogistiKita logistics payment route'),
('umkm_insight', 'dashboard', 'GET', 'http://localhost:8106/umkm_insight/dashboard', FALSE, 'read', 5000, 0, 'analytics:read', TRUE, 'UMKM Insight read-only dashboard route')
ON DUPLICATE KEY UPDATE
    downstream_url = VALUES(downstream_url),
    transactional = VALUES(transactional),
    route_class = VALUES(route_class),
    timeout_ms = VALUES(timeout_ms),
    retry_count = VALUES(retry_count),
    required_scope = VALUES(required_scope),
    is_active = VALUES(is_active),
    description = VALUES(description);
