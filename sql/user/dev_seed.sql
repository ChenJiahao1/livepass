-- 初始化测试用户数据。
INSERT INTO `d_user` (
  `id`, `name`, `rel_name`, `mobile`, `gender`, `password`, `email_status`, `email`,
  `rel_authentication_status`, `id_number`, `address`, `edit_time`, `status`
) VALUES (
  10001, '测试用户', NULL, '13800000000', 1, 'e10adc3949ba59abbe56e057f20f883e', 0, NULL,
  0, NULL, NULL, NOW(), 1
);

-- 初始化用户手机号映射数据。
INSERT INTO `d_user_mobile` (`id`, `user_id`, `mobile`, `edit_time`, `status`) VALUES
(10001, 10001, '13800000000', NOW(), 1);
