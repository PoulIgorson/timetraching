CREATE TABLE IF NOT EXISTS users (
    id             serial PRIMARY KEY,
    pasport_series varchar(4) NOT NULL,
    pasport_number varchar(6) NOT NULL,
    surname        varchar(50),
    name           varchar(50),
    patronymic     varchar(50),
    address        varchar(200),
    created timestamp default now() 
);

CREATE TABLE IF NOT EXISTS tasks (
    id          serial PRIMARY KEY,
    title       varchar(100),
    description varchar(500),
    period_from timestamp,
    period_to   timestamp,
    user_id     int,
    cost        bigint,
    work_from   timestamp,
    created timestamp default now()
);
