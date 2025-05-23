package sql

import (
	"bbutil_cli/server/models"
	"bbutil_cli/server/util"
)

type EmpDao struct{}

func (d *EmpDao) List(param map[string]string) ([]models.EmpInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var emp []models.EmpInfo
	tx = tx.Table("emp_info").Where("emp_info.group_id = ? AND emp_info.status = '1' AND group_info.status = '1'", param["groupId"])
	tx = tx.Select(`
	emp_info.id,
	emp_info.status,
	emp_info.chinese_name,
	emp_info.username,
	emp_info.jenkins_username,
	emp_info.jira_username,
	emp_info.group_id,
	emp_info.address,
	group_info.name as group_name`)
	tx = tx.Joins("LEFT JOIN group_info ON group_info.id = emp_info.group_id").Find(&emp)
	var total int64
	tx.Count(&total)
	if total > 0 {
		return emp, tx.Commit().Error
	} else {
		return nil, tx.Commit().Error
	}

}

func (d *EmpDao) SelectMaxId() (int64, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var maxId int64
	tx.Table("emp_info").Where("emp_info.status = '1'").Select("MAX(id)").Row().Scan(&maxId)
	return maxId, tx.Commit().Error
}

func (d *EmpDao) SelectByUsername(username string) (models.EmpInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var emp models.EmpInfo
	tx.Table("emp_info").Where("emp_info.username = ? ", username).Find(&emp)
	return emp, tx.Commit().Error
}

func (d *EmpDao) SelectByPrimaryKey(id string) (models.EmpInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var emp models.EmpInfo
	tx.Table("emp_info").Where("emp_info.id = ? ", id).Find(&emp)
	return emp, tx.Commit().Error
}

func (d *EmpDao) SelectByUsernameAndAddress(username, address string) (models.EmpInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var emp models.EmpInfo
	tx.Table("emp_info").Where("emp_info.username = ? and emp_info.address = ?", username, address).Find(&emp)
	return emp, tx.Commit().Error
}

func (d *EmpDao) Create(emp models.EmpInfo) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Table("emp_info").Create(&emp).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *EmpDao) Update(emp models.EmpInfo) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	id := emp.Id
	emp.Id = 0
	// emp.JenkinsPassword, _ = util.AESEncoding(emp.JenkinsPassword)
	// emp.JiraPassword, _ = util.AESEncoding(emp.JiraPassword)
	tx = tx.Table("emp_info").Where("emp_info.id = ?", id).Updates(emp)
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *EmpDao) Delete(emp models.EmpInfo) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	tx = tx.Table("emp_info").Where("emp_info.id = ? AND emp_info.group_id = ?", emp.Id, emp.GroupId).Delete(emp)
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
