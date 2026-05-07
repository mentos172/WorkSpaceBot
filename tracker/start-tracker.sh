h

#!/bin/sh

# Ожидаем, пока база данных станет доступна (wb:5432)
./wait-for-it.sh my_postgres:5432 --strict --timeout=60 -- \
    echo "Postgres is up - launching tracker service" &&

# Запускаем сам трекер (замените на вашу фактическую команду запуска)
./tracker