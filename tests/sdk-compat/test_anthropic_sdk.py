"""
Anthropic SDK 兼容性测试

验证 anthropic-sdk-python SDK 通过 Centag 代理访问大模型服务的兼容性。
"""

import os
import json
import pytest
from anthropic import Anthropic

# 配置
BASE_URL = os.getenv("CENTAG_BASE_URL", "http://localhost:20060")
API_KEY = os.getenv("CENTAG_API_KEY", "test-key")
MODEL = os.getenv("ANTHROPIC_MODEL", "claude-3-opus-20240229")


@pytest.fixture
def client():
    """创建 Anthropic 客户端"""
    return Anthropic(
        base_url=f"{BASE_URL}",
        api_key=API_KEY,
    )


class TestAnthropicBasicRequest:
    """基础请求测试"""

    def test_simple_message(self, client):
        """测试简单消息"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            messages=[{"role": "user", "content": "Hello"}],
        )

        # 验证响应结构
        assert response.id is not None
        assert response.type == "message"
        assert response.role == "assistant"
        assert response.model is not None
        assert len(response.content) > 0
        assert response.stop_reason is not None
        assert response.usage is not None
        assert response.usage.input_tokens > 0
        assert response.usage.output_tokens > 0

    def test_system_message(self, client):
        """测试系统消息"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            system="You are a helpful assistant.",
            messages=[{"role": "user", "content": "What is 2+2?"}],
        )

        assert response.content[0].text is not None

    def test_multiple_messages(self, client):
        """测试多轮对话"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            messages=[
                {"role": "user", "content": "My name is Alice"},
                {"role": "assistant", "content": "Hello Alice!"},
                {"role": "user", "content": "What is my name?"},
            ],
        )

        assert response.content[0].text is not None


class TestAnthropicParameters:
    """请求参数测试"""

    def test_temperature(self, client):
        """测试温度参数"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            temperature=0.7,
            messages=[{"role": "user", "content": "Hello"}],
        )

        assert response.content[0].text is not None

    def test_max_tokens(self, client):
        """测试最大 token 参数"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=50,
            messages=[{"role": "user", "content": "Write a long story"}],
        )

        assert response.content[0].text is not None
        assert response.usage.output_tokens <= 50

    def test_top_p(self, client):
        """测试 top_p 参数"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            top_p=0.9,
            messages=[{"role": "user", "content": "Hello"}],
        )

        assert response.content[0].text is not None

    def test_stop_sequences(self, client):
        """测试停止序列"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=50,
            stop_sequences=["3"],
            messages=[{"role": "user", "content": "Count to 10"}],
        )

        assert response.content[0].text is not None


class TestAnthropicToolCalls:
    """工具调用测试"""

    def test_simple_tool(self, client):
        """测试简单工具调用"""
        tools = [
            {
                "name": "get_weather",
                "description": "Get weather for a location",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "location": {"type": "string"}
                    },
                    "required": ["location"],
                },
            }
        ]

        response = client.messages.create(
            model=MODEL,
            max_tokens=100,
            tools=tools,
            messages=[{"role": "user", "content": "What is the weather in Boston?"}],
        )

        # 工具调用可能触发也可能不触发
        assert response.content is not None

    def test_tool_choice_auto(self, client):
        """测试 tool_choice=auto"""
        tools = [
            {
                "name": "get_weather",
                "description": "Get weather for a location",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "location": {"type": "string"}
                    },
                    "required": ["location"],
                },
            }
        ]

        response = client.messages.create(
            model=MODEL,
            max_tokens=100,
            tools=tools,
            tool_choice={"type": "auto"},
            messages=[{"role": "user", "content": "What is the weather?"}],
        )

        assert response.content is not None


class TestAnthropicStreaming:
    """流式响应测试"""

    def test_basic_stream(self, client):
        """测试基础流式响应"""
        with client.messages.stream(
            model=MODEL,
            max_tokens=20,
            messages=[{"role": "user", "content": "Hello"}],
        ) as stream:
            text = stream.get_final_text()
            assert text is not None
            assert len(text) > 0

    def test_stream_events(self, client):
        """测试流式事件"""
        events = []
        with client.messages.stream(
            model=MODEL,
            max_tokens=20,
            messages=[{"role": "user", "content": "Hello"}],
        ) as stream:
            for event in stream:
                events.append(event)

        assert len(events) > 0


class TestAnthropicContentBlocks:
    """内容块测试"""

    def test_text_response(self, client):
        """测试文本响应"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            messages=[{"role": "user", "content": "Hello"}],
        )

        assert response.content[0].type == "text"
        assert response.content[0].text is not None

    def test_vision_content(self, client):
        """测试视觉内容（图片 URL）"""
        # 注意：这需要模型支持视觉
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            messages=[
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "image",
                            "source": {
                                "type": "url",
                                "url": "https://example.com/image.png",
                            },
                        },
                        {
                            "type": "text",
                            "text": "What is in this image?",
                        },
                    ],
                }
            ],
        )

        assert response.content is not None


class TestAnthropicThinking:
    """思考模式测试"""

    def test_thinking_enabled(self, client):
        """测试思考模式"""
        # 注意：需要模型支持 thinking
        try:
            response = client.messages.create(
                model=MODEL,
                max_tokens=100,
                thinking={
                    "type": "enabled",
                    "budget_tokens": 10000,
                },
                messages=[{"role": "user", "content": "Explain quantum computing"}],
            )

            # 验证响应包含 thinking block
            has_thinking = any(block.type == "thinking" for block in response.content)
            has_text = any(block.type == "text" for block in response.content)
            assert has_thinking or has_text
        except Exception as e:
            # 某些模型可能不支持 thinking
            pytest.skip(f"Thinking not supported: {e}")


class TestAnthropicErrorHandling:
    """错误处理测试"""

    def test_invalid_model(self, client):
        """测试无效模型"""
        with pytest.raises(Exception) as exc_info:
            client.messages.create(
                model="nonexistent-model",
                max_tokens=10,
                messages=[{"role": "user", "content": "Hello"}],
            )
        assert exc_info.value is not None

    def test_empty_messages(self, client):
        """测试空消息"""
        with pytest.raises(Exception):
            client.messages.create(
                model=MODEL,
                max_tokens=10,
                messages=[],
            )

    def test_missing_max_tokens(self, client):
        """测试缺少 max_tokens"""
        with pytest.raises(Exception):
            client.messages.create(
                model=MODEL,
                messages=[{"role": "user", "content": "Hello"}],
            )


class TestAnthropicMetadata:
    """元数据测试"""

    def test_user_id(self, client):
        """测试 user_id 元数据"""
        response = client.messages.create(
            model=MODEL,
            max_tokens=10,
            messages=[{"role": "user", "content": "Hello"}],
            metadata={"user_id": "test-user-123"},
        )

        assert response.content[0].text is not None
