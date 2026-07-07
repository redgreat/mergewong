#!/bin/bash

# 进入指定的绝对路径目录
cd /opt/mergewong || { echo "进入目录失败，中止执行"; exit 1; }

echo "=========================================="
echo "    开始重新部署 mergewong 容器环境"
echo "=========================================="

echo "➤ 1. 停止并移除旧容器: mergewong ..."
docker compose stop mergewong
docker compose rm -f mergewong

echo "➤ 2. 删除本地所有的 mergewong 镜像记录..."
# 注意：这会查找带有 mergewong 名称的镜像，并按 ID 强制删除
docker images | grep 'mergewong' | awk '{print $3}' | xargs -r docker rmi -f

echo "➤ 3. 清理所有旧的日志数据..."
# 删除根目录下 ./log/ 内的所有文件及文件夹
rm -rf ./log/*
echo "日志清理完成。"

echo "➤ 4. 重新拉取最新镜像并后台启动..."
docker compose pull mergewong
docker compose up -d mergewong

echo "等待 2 秒检查容器状态..."
sleep 2

echo "➤ 5. 当前 mergewong 容器运行状态:"
docker ps -a --filter "name=mergewong"

echo "=========================================="
echo "    容器已更新并重启完成！"
echo "=========================================="
