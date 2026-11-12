"""工具注册表：负责工具的注册、查找与自动发现。"""
from __future__ import annotations

import importlib
import pkgutil

from app.core.tool import Category, Tool


class ToolRegistry:
    def __init__(self) -> None:
        self._tools: dict[str, Tool] = {}

    def register(self, tool: Tool) -> Tool:
        self._tools[tool.tool_id] = tool
        return tool

    def get(self, tool_id: str) -> Tool | None:
        return self._tools.get(tool_id)

    def all(self, category: Category | None = None) -> list[Tool]:
        tools = list(self._tools.values())
        if category is not None:
            tools = [t for t in tools if t.category == category]
        return sorted(tools, key=lambda t: t.tool_id)

    def categories(self) -> list[Category]:
        return sorted({t.category for t in self._tools.values()}, key=lambda c: c.value)

    def discover(self, package: str = "tools") -> int:
        """自动导入指定包下所有模块，触发 @register_tool 注册。"""
        count = 0
        pkg = importlib.import_module(package)
        for _, modname, _ in pkgutil.walk_packages(pkg.__path__, prefix=package + "."):
            importlib.import_module(modname)
            count += 1
        return count


registry = ToolRegistry()


def register_tool(tool_cls):
    """类装饰器：实例化并注册一个 Tool 子类。"""
    registry.register(tool_cls())
    return tool_cls
