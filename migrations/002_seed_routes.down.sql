DELETE FROM routes_registry
WHERE (service_name, feature_name, method) IN (
    ('smartbank', 'pembayaran_transaksi', 'POST'),
    ('marketplace', 'checkout', 'POST'),
    ('pos', 'pembayaran', 'POST'),
    ('supplierhub', 'pembayaran', 'POST'),
    ('logistikita', 'pembayaran_logistik', 'POST'),
    ('umkm_insight', 'dashboard', 'GET')
);
