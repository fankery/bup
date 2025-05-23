package sql

import (
	"bbutil_cli/server/models"
	"bbutil_cli/server/util"
)

type TaskDao struct{}

// 查询组内整包或替换任务列表
func (t *TaskDao) List(groupId, taskType string) ([]models.TaskInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}

	}()
	var tasks []models.TaskInfo
	tx = tx.Table("task_info").
		Where("task_info.group_id = ? AND task_info.status = 1", groupId)
	if taskType != "" {
		tx = tx.Where("task_info.type = ?", taskType)
	}
	tx.Find(&tasks)
	return tasks, tx.Commit().Error
}

// select by primarykey
func (t *TaskDao) SelectByPrimaryKey(taskId string) (models.TaskInfo, error) {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}

	}()
	var task models.TaskInfo
	tx.Table("task_info").Where("task_info.id = ? AND task_info.status = 1", taskId).Find(&task)
	return task, tx.Commit().Error
}

func (d *TaskDao) Update(task models.TaskInfo) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	id := task.Id
	task.Id = 0

	tx = tx.Table("task_info").Where("task_info.id = ?", id).Updates(task)
	if err := tx.Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *TaskDao) Create(task models.TaskInfo) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Table("task_info").Create(&task).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (d *TaskDao) Delete(id string) error {
	tx := util.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Table("task_info").Where("id = ?", id).Delete(&models.TaskInfo{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
