package mysql

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"tcc_transaction/log"
	"tcc_transaction/store/data"
	"time"
)

const (
	maxOpenConns = 10
	maxIdleConns = 10
	maxLifeTime  = 300
)

type MysqlClient struct {
	c *sqlx.DB
}

// tcc:tcc_123@tcp(localhost:3306)/tcc?charset=utf8
func NewMysqlClient(user, pwd, host, port, database string) *MysqlClient {
	db, err := sqlx.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", user, pwd, host, port, database))
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(maxLifeTime * time.Second)
	return &MysqlClient{
		c: db,
	}
}

func (c *MysqlClient) InsertRequestInfo(r *data.RequestInfo) (error) {
	sql := `INSERT INTO request_info (url, method, param) values(?, ?, ?)`
	rst, err := c.c.Exec(sql, r.Url, r.Method, r.Param)
	if err != nil {
		return err
	}

	id, err := rst.LastInsertId()
	if err != nil {
		return err
	}
	r.Id = id
	return nil
}

func (c *MysqlClient) UpdateRequestInfoStatus(status int, id int64) error {
	sql := `UPDATE request_info SET status = ? where id = ?`
	_, err := c.c.Exec(sql, status, id)
	return err
}

func (c *MysqlClient) UpdateRequestInfoTimes(id int64) error {
	sql := `UPDATE request_info SET times = times + 1 where id = ?`
	_, err := c.c.Exec(sql, id)
	return err
}

func (c *MysqlClient) UpdateRequestInfoSend(id int64) error {
	sql := `UPDATE request_info SET is_send = 1 where id = ?`
	_, err := c.c.Exec(sql, id)
	return err
}

func (c *MysqlClient) ListExceptionalRequestInfo() ([]*data.RequestInfo, error) {
	var rst []*data.RequestInfo
	sql := `SELECT id, 
				   url, 
				   method, 
				   param, 
				   status, 
				   times, 
				   is_send, 
				   deleted 
			  FROM request_info
			 WHERE status in (2, 4) 
			   AND deleted = 0`

	err := c.c.Select(&rst, sql)
	if err != nil {
		return nil, err
	}
	sql = `SELECT id, request_id, idx, url, method, status, try_result, param FROM success_step WHERE request_id = ? AND status = 0 AND deleted = 0`
	for idx, ri := range rst {
		var ss []*data.SuccessStep
		err = c.c.Select(&ss, sql, ri.Id)
		if err != nil {
			log.Errorf("read success step info failed from mysql, please check it. error info is: %s", err)
			continue
		}
		rst[idx].SuccessSteps = ss
	}
	return rst, nil
}

func (c *MysqlClient) InsertSuccessStep(s *data.SuccessStep) (error) {
	sql := `INSERT INTO success_step (request_id, idx, status, url, method, param, try_result) values(?, ?, ?, ?, ?, ?, ?)`
	rst, err := c.c.Exec(sql, s.RequestId, s.Index, s.Status, s.Url, s.Method, s.Param, s.Result)
	if err != nil {
		return err
	}
	id, err := rst.LastInsertId()
	if err != nil {
		return err
	}
	s.Id = id
	return nil
}

func (c *MysqlClient) UpdateSuccessStepStatus(id int64, status int) error {
	sql := `UPDATE success_step SET status = ? WHERE id = ?`
	_, err := c.c.Exec(sql, id, status)
	return err
}

func (c *MysqlClient) Confirm(id int64) error {
	tx, err := c.c.Begin()
	if err != nil {
		return err
	}
	sql := `UPDATE success_step SET status = 1 WHERE request_id = ?`
	_, err = tx.Exec(sql, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	sql = `UPDATE request_info SET status = 1 WHERE id = ?`
	_, err = tx.Exec(sql, id)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return err
}
