package sql

import (
	"bbutil_cli/server/models"
	"bbutil_cli/server/util"
)

type GroupDao struct{}

func (d *GroupDao) List() ([]models.GroupInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}

	}()
	var groups []models.GroupInfo
	tx.Table("group_info").Select("id, name,status, all_web_hook, replace_web_hook, root_name, chan").Where("status = 1").Find(&groups)
	return groups, tx.Commit().Error
}

func (d *GroupDao) SelectByPrimaryKey(id string) (models.GroupInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var group models.GroupInfo
	tx.Table("group_info").Select("id, status, name, all_web_hook, replace_web_hook, root_name, chan").Where("id = ?", id).Find(&group)
	return group, tx.Commit().Error
}

func (d *GroupDao) Create(group models.GroupInfo) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Table("group_info").Create(&group).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *GroupDao) Delete(id string) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Table("group_info").Where("id = ?", id).Delete(&models.GroupInfo{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *GroupDao) Update(group models.GroupInfo) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	id := group.Id
	group.Id = 0
	tx = tx.Table("group_info").Where("group_info.id = ?", id).Updates(group)
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
