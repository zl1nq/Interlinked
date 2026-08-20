-- Feed流系统数据库初始化脚本
-- 请先创建数据库，然后运行后端程序自动迁移表结构

CREATE DATABASE IF NOT EXISTS feed_system 
  CHARACTER SET utf8mb4 
  COLLATE utf8mb4_unicode_ci;

USE feed_system;