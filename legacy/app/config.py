from functools import lru_cache
from pathlib import Path

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_prefix="CTF_", extra="ignore")

    app_name: str = "CTF Toolbox"
    api_prefix: str = "/api"
    data_dir: Path = Path("data")
    default_timeout: int = 60
    max_timeout: int = 600

    # 执行引擎后端：local（本机子进程）| docker（沙箱隔离）
    executor_backend: str = "local"
    docker_image: str = "ctf-toolbox:latest"
    docker_network: str = "none"
    # 命名卷名：设置后 DockerExecutor 用命名卷挂载（App 容器化 / DooD 场景）
    docker_data_volume: str | None = None

    @property
    def artifacts_dir(self) -> Path:
        return self.data_dir / "artifacts"

    @property
    def work_dir(self) -> Path:
        return self.data_dir / "work"

    @property
    def uploads_dir(self) -> Path:
        return self.data_dir / "uploads"


@lru_cache
def get_settings() -> Settings:
    return Settings()
