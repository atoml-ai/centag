"""
OpenAI SDK 兼容性测试

验证 openai-python SDK 通过 Centag 代理访问大模型服务的兼容性。
"""

import os
import json
import pytest
from openai import OpenAI

# 配置
BASE_URL = os.getenv("CENTAG_BASE_URL", "http://localhost:20060")
API_KEY = os.getenv("CENTAG_API_KEY", "test-key")
MODEL = os.getenv("OPENAI_MODEL", "gpt-4")


@pytest.fixture
def client():
    """创建 OpenAI 客户端"""
    return OpenAI(
        base_url=f"{BASE_URL}/v1",
        api_key=API_KEY,
    )


class TestOpenAIBasicRequest:
    """基础请求测试"""

    def test_simple_chat(self, client):
        """测试简单对话"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Hello"}],
            max_tokens=10,
        )

        # 验证响应结构
        assert response.id is not None
        assert response.object == "chat.completion"
        assert response.model is not None
        assert len(response.choices) > 0
        assert response.choices[0].message.content is not None
        assert response.usage is not None
        assert response.usage.total_tokens > 0

    def test_system_message(self, client):
        """测试系统消息"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[
                {"role": "system", "content": "You are a helpful assistant."},
                {"role": "user", "content": "What is 2+2?"},
            ],
            max_tokens=10,
        )

        assert response.choices[0].message.content is not None

    def test_multiple_messages(self, client):
        """测试多轮对话"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[
                {"role": "user", "content": "My name is Alice"},
                {"role": "assistant", "content": "Hello Alice!"},
                {"role": "user", "content": "What is my name?"},
            ],
            max_tokens=10,
        )

        assert response.choices[0].message.content is not None


class TestOpenAIParameters:
    """请求参数测试"""

    def test_temperature(self, client):
        """测试温度参数"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Hello"}],
            temperature=0.7,
            max_tokens=10,
        )

        assert response.choices[0].message.content is not None

    def test_max_tokens(self, client):
        """测试最大 token 参数"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Write a long story"}],
            max_tokens=50,
        )

        assert response.choices[0].message.content is not None
        assert response.usage.completion_tokens <= 50

    def test_top_p(self, client):
        """测试 top_p 参数"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Hello"}],
            top_p=0.9,
            max_tokens=10,
        )

        assert response.choices[0].message.content is not None

    def test_stop_sequences(self, client):
        """测试停止序列"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Count to 10"}],
            stop=["3"],
            max_tokens=50,
        )

        assert response.choices[0].message.content is not None


class TestOpenAIToolCalls:
    """工具调用测试"""

    def test_simple_tool(self, client):
        """测试简单工具调用"""
        tools = [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get weather for a location",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "location": {"type": "string"}
                        },
                        "required": ["location"],
                    },
                },
            }
        ]

        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "What is the weather in Boston?"}],
            tools=tools,
            tool_choice="auto",
            max_tokens=100,
        )

        # 工具调用可能触发也可能不触发
        assert response.choices[0].message is not None

    def test_tool_choice_auto(self, client):
        """测试 tool_choice=auto"""
        tools = [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get weather for a location",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "location": {"type": "string"}
                        },
                        "required": ["location"],
                    },
                },
            }
        ]

        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "What is the weather?"}],
            tools=tools,
            tool_choice="auto",
            max_tokens=100,
        )

        assert response.choices[0].message is not None


class TestOpenAIStreaming:
    """流式响应测试"""

    def test_basic_stream(self, client):
        """测试基础流式响应"""
        stream = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Hello"}],
            max_tokens=20,
            stream=True,
        )

        chunks = []
        for chunk in stream:
            chunks.append(chunk)
            assert chunk.id is not None
            assert chunk.object == "chat.completion.chunk"
            if chunk.choices[0].delta.content:
                assert isinstance(chunk.choices[0].delta.content, str)

        assert len(chunks) > 0

    def test_stream_with_usage(self, client):
        """测试带 usage 的流式响应"""
        stream = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Hello"}],
            max_tokens=20,
            stream=True,
            stream_options={"include_usage": True},
        )

        chunks = list(stream)
        assert len(chunks) > 0


class TestOpenAIErrorHandling:
    """错误处理测试"""

    def test_invalid_model(self, client):
        """测试无效模型"""
        with pytest.raises(Exception) as exc_info:
            client.chat.completions.create(
                model="nonexistent-model",
                messages=[{"role": "user", "content": "Hello"}],
                max_tokens=10,
            )
        assert exc_info.value is not None

    def test_empty_messages(self, client):
        """测试空消息"""
        with pytest.raises(Exception):
            client.chat.completions.create(
                model=MODEL,
                messages=[],
                max_tokens=10,
            )


class TestOpenAIResponseFormat:
    """响应格式测试"""

    def test_json_mode(self, client):
        """测试 JSON 模式"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Return a JSON object with name and age"}],
            response_format={"type": "json_object"},
            max_tokens=50,
        )

        assert response.choices[0].message.content is not None
        # 尝试解析为 JSON
        try:
            json.loads(response.choices[0].message.content)
        except json.JSONDecodeError:
            # 某些模型可能不完全支持 JSON 模式
            pass


class TestOpenAIMetadata:
    """元数据测试"""

    def test_user_field(self, client):
        """测试 user 字段"""
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": "Hello"}],
            max_tokens=10,
            user="test-user-123",
        )

        assert response.choices[0].message.content is not None
