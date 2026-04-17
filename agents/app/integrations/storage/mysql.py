"""MySQL connection factory for agents storage adapters."""

import pymysql

from app.shared.config import Settings, get_settings


class MySQLConnectionFactory:
    def __init__(self, settings: Settings | None = None) -> None:
        self.settings = settings or get_settings()

    def connect(self):
        return pymysql.connect(
            host=self.settings.agents_mysql_host,
            port=self.settings.agents_mysql_port,
            user=self.settings.agents_mysql_user,
            password=self.settings.agents_mysql_password,
            database=self.settings.agents_mysql_database,
            charset=self.settings.agents_mysql_charset,
            cursorclass=pymysql.cursors.DictCursor,
            autocommit=False,
        )
