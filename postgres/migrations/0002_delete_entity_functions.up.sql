CREATE OR REPLACE FUNCTION delete_namespace_and_nkeys(namespace_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete user nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM "user" u WHERE u.namespace_id = delete_namespace_and_nkeys.namespace_id);

    -- delete account nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM account a WHERE a.namespace_id = delete_namespace_and_nkeys.namespace_id);

    -- delete operator nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM operator o WHERE o.namespace_id = delete_namespace_and_nkeys.namespace_id);

    -- delete the namespace (cascading operators, accounts, and users)
    DELETE FROM namespace WHERE id = delete_namespace_and_nkeys.namespace_id;

END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_operator_and_nkeys(operator_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete user nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM "user" u WHERE u.operator_id = delete_operator_and_nkeys.operator_id);

    -- delete account nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM account a WHERE a.operator_id = delete_operator_and_nkeys.operator_id);

    -- delete the operator nkey
    DELETE FROM nkey WHERE id = delete_operator_and_nkeys.operator_id;

    -- delete the operator (cascading accounts and users)
    DELETE FROM operator WHERE id = delete_operator_and_nkeys.operator_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_account_and_nkeys(account_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete user nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM "user" u WHERE u.account_id = delete_account_and_nkeys.account_id);

    -- delete the account nkey
    DELETE FROM nkey WHERE id = delete_account_and_nkeys.account_id;

    -- delete the account (cascading users)
    DELETE FROM account WHERE id = delete_account_and_nkeys.account_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_user_and_nkey(user_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete the user nkey
    DELETE FROM nkey WHERE id = delete_user_and_nkey.user_id;

    -- delete the user
    DELETE FROM "user" WHERE id = delete_user_and_nkey.user_id;
END;
$$ LANGUAGE plpgsql;