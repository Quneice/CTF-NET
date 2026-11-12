class ToolboxError(Exception):
    """工具箱领域异常基类。"""


class ToolNotFoundError(ToolboxError):
    pass


class ToolValidationError(ToolboxError):
    pass
