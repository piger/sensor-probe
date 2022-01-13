CREATE TABLE IF NOT EXISTS home_temperature (
  time TIMESTAMP NOT NULL,
  room text NOT NULL,
  temperature double PRECISION NULL,
  humidity double PRECISION NULL,
  battery double PRECISION NULL
);

SELECT create_hypertable('home_temperature', 'time');
