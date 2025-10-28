-- remove nkey when a user is deleted (directly or via cascade)
CREATE TRIGGER trg_user_delete_nkey
    AFTER DELETE ON "user"
    FOR EACH ROW
BEGIN
DELETE FROM nkey
WHERE id = OLD.id
  AND type = 'user';
END;

--  remove nkey when an account is deleted (directly or via cascade)
CREATE TRIGGER trg_account_delete_nkey
    AFTER DELETE ON account
    FOR EACH ROW
BEGIN
DELETE FROM nkey
WHERE id = OLD.id
  AND type = 'account';
END;

--  remove nkey when an operator is deleted (directly or via cascade)
CREATE TRIGGER trg_operator_delete_nkey
    AFTER DELETE ON operator
    FOR EACH ROW
BEGIN
DELETE FROM nkey
WHERE id = OLD.id
  AND type = 'operator';
END;