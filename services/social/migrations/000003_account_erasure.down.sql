UPDATE comments SET content = ' ' WHERE content = '';
ALTER TABLE comments DROP CONSTRAINT comments_content_chk;
ALTER TABLE comments ADD CONSTRAINT comments_content_chk CHECK (char_length(content) BETWEEN 1 AND 1000);

DROP TABLE erased_users;
