CREATE TABLE IF NOT EXISTS routes_registry (
    id INT AUTO_INCREMENT PRIMARY KEY,
    service_name VARCHAR(50) NOT NULL,
    feature_name VARCHAR(100) NOT NULL,
    method VARCHAR(10) NOT NULL DEFAULT 'POST',
    downstream_url VARCHAR(500) NOT NULL,
    transactional BOOLEAN NOT NULL DEFAULT FALSE,
    route_class ENUM('read','transactional') NOT NULL DEFAULT 'read',
    timeout_ms INT NOT NULL DEFAULT 5000,
    retry_count INT NOT NULL DEFAULT 0,
    required_scope VARCHAR(255) NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_route (service_name, feature_name, method),
    INDEX idx_active (is_active)
);

CREATE TABLE IF NOT EXISTS request_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    request_id VARCHAR(36) NOT NULL,
    parent_request_id VARCHAR(36) NULL,
    user_id VARCHAR(50) NULL,
    source_app VARCHAR(50) NULL,
    target_app VARCHAR(50) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    status_code INT NULL,
    latency_ms INT NULL,
    ip_address VARCHAR(45) NOT NULL,
    request_hash VARCHAR(64) NULL,
    response_hash VARCHAR(64) NULL,
    lifecycle ENUM('STARTED','COMPLETED','FAILED') NOT NULL,
    error_message TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_request_id (request_id),
    INDEX idx_user_id (user_id),
    INDEX idx_target_app (target_app),
    INDEX idx_created_at (created_at),
    INDEX idx_status_code (status_code)
);

CREATE TABLE IF NOT EXISTS gateway_fees (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    request_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(50) NOT NULL,
    source_app VARCHAR(50) NOT NULL,
    transaction_amount BIGINT NOT NULL,
    fee_amount BIGINT NOT NULL,
    fee_rate DECIMAL(5,4) NOT NULL DEFAULT 0.0050,
    status ENUM('SUCCESS','PENDING','FAILED') NOT NULL DEFAULT 'PENDING',
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 5,
    next_retry_at DATETIME NULL,
    smartbank_ref VARCHAR(100) NULL,
    error_message TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_fee_request (request_id),
    INDEX idx_status (status),
    INDEX idx_user_id (user_id),
    INDEX idx_next_retry (next_retry_at)
);

CREATE TABLE IF NOT EXISTS jwt_blacklist (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    jti VARCHAR(255) NOT NULL,
    user_id VARCHAR(50) NOT NULL,
    reason VARCHAR(255) NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_jti (jti),
    INDEX idx_expires (expires_at)
);

CREATE TABLE IF NOT EXISTS operators (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    role ENUM('AdminFull','Operator','FinanceAuditor','ReadOnlyViewer') NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_email (email)
);
